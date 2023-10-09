package dolby

import (
	"fmt"
	"io"
	"net/http"
)

func main() {

	url := "https://director.millicast.com/api/whip/streamName?codec=h264"

	req, _ := http.NewRequest("POST", url, nil)

	req.Header.Add("accept", "application/sdp")
	req.Header.Add("content-type", "application/sdp")

	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	fmt.Println(string(body))

}
