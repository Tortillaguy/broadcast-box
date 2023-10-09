package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

type (
	whepSession struct {
		videoTrack     *trackMultiCodec
		currentLayer   atomic.Value
		sequenceNumber uint16
		timestamp      uint32
	}

	simulcastLayerResponse struct {
		EncodingId string `json:"encodingId"`
	}
)

func WHEPLayers(whepSessionId string) ([]byte, error) {
	streamMapLock.Lock()
	defer streamMapLock.Unlock()

	layers := []simulcastLayerResponse{}
	for streamKey := range streamMap {
		streamMap[streamKey].whepSessionsLock.Lock()
		defer streamMap[streamKey].whepSessionsLock.Unlock()

		if _, ok := streamMap[streamKey].whepSessions[whepSessionId]; ok {
			for i := range streamMap[streamKey].videoTrackLabels {
				layers = append(layers, simulcastLayerResponse{EncodingId: streamMap[streamKey].videoTrackLabels[i]})
			}

			break
		}
	}

	resp := map[string]map[string][]simulcastLayerResponse{
		"1": map[string][]simulcastLayerResponse{
			"layers": layers,
		},
	}

	return json.Marshal(resp)
}

func WHEPChangeLayer(whepSessionId, layer string) error {
	streamMapLock.Lock()
	defer streamMapLock.Unlock()

	for streamKey := range streamMap {
		streamMap[streamKey].whepSessionsLock.Lock()
		defer streamMap[streamKey].whepSessionsLock.Unlock()

		if _, ok := streamMap[streamKey].whepSessions[whepSessionId]; ok {
			streamMap[streamKey].whepSessions[whepSessionId].currentLayer.Store(layer)
			streamMap[streamKey].pliChan <- true
		}
	}

	return nil
}

func WHEP(offer, streamKey string) (string, string, error) {
	streamMapLock.Lock()
	defer streamMapLock.Unlock()
	stream, err := getStream(streamKey)

	if err != nil {
		return "", "", err
	}

	whepSessionId := uuid.New().String()

	videoTrack := &trackMultiCodec{id: "video", streamID: "pion"}

	// log.Println(videoTrack->)

	peerConnection, err := apiWhep.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return "", "", err
	}

	peerConnection.OnICEConnectionStateChange(func(i webrtc.ICEConnectionState) {
		if i == webrtc.ICEConnectionStateFailed {
			if err := peerConnection.Close(); err != nil {
				log.Println(err)
			}

			stream.whepSessionsLock.Lock()
			defer stream.whepSessionsLock.Unlock()
			delete(stream.whepSessions, whepSessionId)
		}
	})

	// if _, err = peerConnection.AddTrack(stream.audioTrack); err != nil {
	// 	return "", "", err
	// }

	rtpSender, err := peerConnection.AddTrack(videoTrack)

	// log.Println(rtpSender.GetParameters().Encodings)
	// videoTrack.rid = "2"
	// err = rtpSender.AddEncoding()

	// log.Println("RID", stream.audioTrack.RID())

	// _, err: rtpSender.AddEncoding(videoTrack)
	// err = rtpSender.AddEncoding(stream.audioTrack)

	if err != nil {
		log.Println(err)
	}

	// log.Println(rtpSender.GetParameters().Codecs)
	if err != nil {
		log.Println("bad rtp sender")
		return "", "", err
	}

	go func() {
		for {
			rtcpPackets, _, rtcpErr := rtpSender.ReadRTCP()
			if rtcpErr != nil {
				return
			}

			for _, r := range rtcpPackets {
				if _, isPLI := r.(*rtcp.PictureLossIndication); isPLI {
					select {
					case stream.pliChan <- true:
					default:
					}
				}
			}
		}
	}()

	if err := peerConnection.SetRemoteDescription(webrtc.SessionDescription{
		SDP:  offer,
		Type: webrtc.SDPTypeOffer,
	}); err != nil {
		return "", "", err
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// log.Println("creating answer")
	answer, err := peerConnection.CreateAnswer(nil)
	// log.Println(answer)

	if err != nil {
		log.Println("Error is logged here")
		return "", "", err
	} else if err = peerConnection.SetLocalDescription(answer); err != nil {
		log.Println("error setting local description")
		return "", "", err
	}

	<-gatherComplete

	stream.whepSessionsLock.Lock()
	defer stream.whepSessionsLock.Unlock()

	stream.whepSessions[whepSessionId] = &whepSession{
		videoTrack: videoTrack,
		timestamp:  50000,
	}
	stream.whepSessions[whepSessionId].currentLayer.Store("")
	return peerConnection.LocalDescription().SDP, whepSessionId, nil
}

func (w *whepSession) sendVideoPacket(rtpPkt *rtp.Packet, layer string, timeDiff uint32, isAV1 bool) {
	if w.currentLayer.Load() == "" {
		w.currentLayer.Store(layer)
	} else if layer != w.currentLayer.Load() {
		return
	}

	w.sequenceNumber += 1
	w.timestamp += timeDiff

	rtpPkt.SequenceNumber = w.sequenceNumber
	rtpPkt.Timestamp = w.timestamp

	if err := w.videoTrack.WriteRTP(rtpPkt, isAV1); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		log.Println(err)
	}
}
