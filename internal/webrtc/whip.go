package main

import (
	"errors"
	"io"
	"log"
	"strings"

	"github.com/glimesh/broadcast-box/internal/udp"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

func audioWriter(remoteTrack *webrtc.TrackRemote, audioTrack *webrtc.TrackLocalStaticRTP) {
	rtpBuf := make([]byte, 1500)
	for {
		rtpRead, _, err := remoteTrack.Read(rtpBuf)
		switch {
		case errors.Is(err, io.EOF):
			return
		case err != nil:
			log.Println(err)
			return
		}

		if _, writeErr := audioTrack.Write(rtpBuf[:rtpRead]); writeErr != nil && !errors.Is(writeErr, io.ErrClosedPipe) {
			log.Println(writeErr)
			return
		}
	}
}

func videoWriter(remoteTrack *webrtc.TrackRemote, stream *stream, peerConnection *webrtc.PeerConnection, s *stream) {
	id := remoteTrack.RID()
	if id == "" {
		id = videoTrackLabelDefault
	}
	log.Println(id)

	if err := addTrack(s, id); err != nil {
		log.Println(err)
		return
	}

	go func() {
		for range stream.pliChan {
			if sendErr := peerConnection.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{
					MediaSSRC: uint32(remoteTrack.SSRC()),
				},
			}); sendErr != nil {
				return
			}
		}
	}()

	isAV1 :=
		strings.Contains(
			strings.ToLower(webrtc.MimeTypeAV1),
			strings.ToLower(remoteTrack.Codec().RTPCodecCapability.MimeType),
		)

	rtpBuf := make([]byte, 1500)
	rtpPkt := &rtp.Packet{}
	lastTimestamp := uint32(0)

	// Modify the frame here (similar to JavaScript code)
	frame := rtpBuf
	message := []byte(udp.GetPayload())
	start := []byte{0xCA, 0xFE, 0xBA, 0xBE}
	data := append(start, message...)
	data = append(data, []byte{0, 0, 0, byte(len(message))}...)

	// Create a new buffer to hold modified frame
	newFrame := make([]byte, len(frame)+len(data))
	copy(newFrame, frame)
	copy(newFrame[len(frame):], data)

	rtpBuf = frame

	for {
		rtpRead, _, err := remoteTrack.Read(rtpBuf)
		switch {
		case errors.Is(err, io.EOF):
			return
		case err != nil:
			log.Println(err)
			return
		}

		if err = rtpPkt.Unmarshal(rtpBuf[:rtpRead]); err != nil {
			log.Println(err)
			return
		}

		timeDiff := rtpPkt.Timestamp - lastTimestamp
		if lastTimestamp == 0 {
			timeDiff = 0
		}
		lastTimestamp = rtpPkt.Timestamp

		s.whepSessionsLock.RLock()
		for i := range s.whepSessions {
			// log.Println("video packet", id, timeDiff, rtpPkt.PaddingSize, rtpPkt.Timestamp)
			// log.Println(len(rtpPkt.Payload))
			s.whepSessions[i].sendVideoPacket(rtpPkt, id, timeDiff, isAV1)
		}
		s.whepSessionsLock.RUnlock()
	}
}

func WHIP(offer, streamKey string) (string, error) {
	peerConnection, err := apiWhip.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return "", err
	}

	streamMapLock.Lock()
	defer streamMapLock.Unlock()
	stream, err := getStream(streamKey)
	if err != nil {
		return "", err
	}

	peerConnection.OnTrack(func(remoteTrack *webrtc.TrackRemote, rtpReceiver *webrtc.RTPReceiver) {
		if strings.HasPrefix(remoteTrack.Codec().RTPCodecCapability.MimeType, "audio") {
			audioWriter(remoteTrack, stream.audioTrack)
		} else {
			videoWriter(remoteTrack, stream, peerConnection, stream)
		}
	})

	peerConnection.OnICEConnectionStateChange(func(i webrtc.ICEConnectionState) {
		if i == webrtc.ICEConnectionStateFailed {
			if err := peerConnection.Close(); err != nil {
				log.Println(err)
			}
			deleteStream(streamKey)
		}
	})

	if err := peerConnection.SetRemoteDescription(webrtc.SessionDescription{
		SDP:  string(offer),
		Type: webrtc.SDPTypeOffer,
	}); err != nil {
		return "", err
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	answer, err := peerConnection.CreateAnswer(nil)

	if err != nil {
		return "", err
	} else if err = peerConnection.SetLocalDescription(answer); err != nil {
		return "", err
	}

	<-gatherComplete
	return peerConnection.LocalDescription().SDP, nil
}

func GetAllStreams() (out []string) {
	streamMapLock.Lock()
	defer streamMapLock.Unlock()

	for s := range streamMap {
		out = append(out, s)
	}

	return
}
