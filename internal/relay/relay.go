package relay

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/joho/godotenv"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"

	internal "github.com/glimesh/broadcast-box/internal/webrtc"
)

var payload = ""

func InitRelay(whepClient *webrtc.API) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	endpoint := os.Getenv("Endpoint")
	token := os.Getenv("Token")

	pc, err := whepClient.NewPeerConnection(webrtc.Configuration{})

	videoTrack := internal.GetTrackMultiCodec()

	pc.AddTransceiverFromTrack(videoTrack, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly})

	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Printf("PeerConnection State has changed %s \n", connectionState.String())
	})

	// TODO: Set up handler from other stream and relay here

	// Continue setup
	offer, err := pc.CreateOffer(nil)

	err = pc.SetLocalDescription(offer)
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	<-gatherComplete

	var sdp = []byte(pc.LocalDescription().SDP)
	client := &http.Client{}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(sdp))
	req.Header.Add("Content-Type", "application/sdp")
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	if resp.StatusCode != 201 {
		log.Fatalf("Non Successful POST: %d", resp.StatusCode)
	}

	resourceUrl, err := url.Parse(resp.Header.Get("Location"))
	base, err := url.Parse(endpoint)

	resourceUrl = base.ResolveReference(resourceUrl)
	answer := webrtc.SessionDescription{}
	answer.Type = webrtc.SDPTypeAnswer
	answer.SDP = string(body)

	err = pc.SetRemoteDescription(answer)
}

func EmbedMetadata(rtpPkt *rtp.Packet) []byte {
	rtpBuf := make([]byte, 1500)

	// Modify the frame here
	frame := rtpBuf
	message := []byte(payload)
	start := []byte{0xCA, 0xFE, 0xBA, 0xBE}
	data := append(start, message...)
	data = append(data, []byte{0, 0, 0, byte(len(message))}...)

	// Create a new buffer to hold modified frame
	newFrame := make([]byte, len(frame)+len(data))
	copy(newFrame, frame)
	copy(newFrame[len(frame):], data)

	rtpBuf = frame

	return rtpBuf
}
