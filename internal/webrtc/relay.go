package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/pion/webrtc/v3"
)

var (
	endpointURL string = "https://director.millicast.com/api/whip/myStreamName"
	token       string = "1f72732cc2b802f199d59f16e0ccd66580d0b75c34d57bd066e85a5d35beb03b"
	resourceUrl *url.URL
)

func logError(err error) {
	if err != nil {
		log.Println(err)
	}
}

func main() {
	c := make(chan bool)

	Configure()
	peerConnection, err := apiWhep.NewPeerConnection(webrtc.Configuration{})
	logError(err)

	// sessionDescription, err := peerConnection.CreateOffer(&webrtc.OfferOptions{})

	videoTrack := &trackMultiCodec{id: "video", streamID: "pion"}
	peerConnection.AddTransceiverFromTrack(videoTrack, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		// c <- true
		log.Printf("PeerConnection State has changed %s \n", connectionState.String())
	})

	// TODO: Set up handler from other stream and relay here

	// Continue setup
	offer, err := peerConnection.CreateOffer(nil)

	logError(err)

	err = peerConnection.SetLocalDescription(offer)

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	<-gatherComplete

	var sdp = []byte(peerConnection.LocalDescription().SDP)
	// client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	client := &http.Client{}

	req, err := http.NewRequest("POST", endpointURL, bytes.NewBuffer(sdp))
	logError(err)

	req.Header.Add("Content-Type", "application/sdp")
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	logError(err)

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		log.Fatalf("Non Successful POST: %d", resp.StatusCode)
	}

	resourceUrl, err = url.Parse(resp.Header.Get("Location"))
	logError(err)

	base, err := url.Parse(endpointURL)
	logError(err)

	resourceUrl = base.ResolveReference(resourceUrl)

	answer := webrtc.SessionDescription{}
	answer.Type = webrtc.SDPTypeAnswer
	answer.SDP = string(body)

	err = peerConnection.SetRemoteDescription(answer)
	logError(err)

	<-c
}
