import React, { useContext, useEffect, useMemo, useState } from 'react'
import { parseLinkHeader } from '@web3-storage/parse-link-header'
import { useLocation } from 'react-router-dom'
import { Director, Publish } from '@millicast/sdk';
// import  {EncodedStream}  from 'mediastream-to-webm'

export const CinemaModeContext = React.createContext(null);

export function CinemaModeProvider({ children }) {
  const [cinemaMode, setCinemaMode] = useState(() => localStorage.getItem("cinema-mode") === "true");
  const state = useMemo(() => ({
    cinemaMode,
    setCinemaMode,
    toggleCinemaMode: () => setCinemaMode((prev) => !prev),
  }), [cinemaMode, setCinemaMode]);

  useEffect(() => localStorage.setItem("cinema-mode", cinemaMode), [cinemaMode]);
  return (
    <CinemaModeContext.Provider value={state}>
      {children}
    </CinemaModeContext.Provider>
  );
}

function PlayerPage() {
  const { cinemaMode, toggleCinemaMode } = useContext(CinemaModeContext);
  return (
    <div className={`flex flex-col items-center ${!cinemaMode && 'mx-auto px-2 py-2 container'}`}>
      <Player cinemaMode={cinemaMode} />
      <button className='bg-blue-900 px-4 py-2 rounded-lg mt-6' onClick={toggleCinemaMode}>
        {cinemaMode ? "Disable cinema mode" : "Enable cinema mode"}
      </button>
    </div>
  )
}
const tokenGenerator = () => Director.getPublisher(
  {
    token: 'my-publishing-token', 
    streamName: 'my-stream-name'
  });

const publisher = new Publish('my-stream-name', tokenGenerator);


function Player({ cinemaMode }) {
  const videoRef = React.createRef()
  const location = useLocation()
  const [videoLayers, setVideoLayers] = React.useState([]);
  const [mediaSrcObject, setMediaSrcObject] = React.useState(null);
  const [layerEndpoint, setLayerEndpoint] = React.useState('');

  const onLayerChange = event => {
    fetch(layerEndpoint, {
      method: 'POST',
      body: JSON.stringify({ mediaId: '1', encodingId: event.target.value }),
      headers: {
        'Content-Type': 'application/json'
      }
    })
  }

  React.useEffect(() => {
    if (videoRef.current) {
      videoRef.current.srcObject = mediaSrcObject
    }
  }, [mediaSrcObject, videoRef])

  React.useEffect(() => {
    const peerConnection = new RTCPeerConnection() 
    const s = new MediaStream()

    // const audio = peerConnection.addTransceiver('audio', {direction: 'recvonly'})
    const video = peerConnection.addTransceiver('video', { direction:'recvonly'})
    // video.setCodecPreferences(RTCRtpReceiver.getCapabilities('video').codecs)
    // audio.setCodecPreferences(RTCRtpReceiver.getCapabilities('audio').codecs)
    // video.receiver.createEncodedStreams()
    console.log(peerConnection.currentLocalDescription)

    peerConnection.ontrack = async function (event) {
      const encodedStream = event.streams[0]
      // Create encoded stream and transformer
      // encodedStream.getVideoTracks()[0
      console.log(event)
      console.log({transform: event.receiver.transform})
      // const p = event.receiver.createEncodedStreams();
      // event.receiver.transform = new RTCRtpScriptTransform()
      const transformer = new TransformStream({
        async transform(frame, controller) {
            // Convert data from ArrayBuffer to Uint8Array
            const frame_data = new Uint8Array(frame.data);
            const total_length = frame_data.length;

            // Shift to left for endianess to retrieve the metadata size from the last
            // 4 bytes of the buffer
            let shft = 3;
            const size = frame_data.slice(total_length - 4)
                .reduce((acc, v) => acc + (v << shft--), 0);
 
            // Use the byte signal identifying that the remaining data is frame metadata and
            // confirm that the signal is in the frame.
            const magic_value = [ 0xCA, 0xFE, 0xBA, 0xBE ];
            const magic_bytes = frame_data.slice(total_length - size - 2*4, total_length - size - 4);
            let has_magic_value = magic_value.every((v, index) => v === magic_bytes[index]);
 
            // When there is metadata in the frame, get the metadata array and handle it
            // as needed by your application.
            if(has_magic_value) {
                const data = frame_data.slice(total_length - size - 4, total_length - 4);
                console.log("received data : ", String.fromCharCode(...data));
            }

          // Send the frame as is which is supported by video players
            controller.enqueue(frame);
        },
    });
 
    // encodedStream.readable.pipeThrough(transformer)
    // .pipeTo(encodedStream.writable);
      
      setMediaSrcObject(encodedStream)
    }

    // const availReceiveCodecs = RTCRtpReceiver.getCapabilities("video").codecs;
    // console.log(availReceiveCodecs)

    peerConnection.createOffer({offerToReceiveAudio: false, offerToReceiveVideo: true}).then(offer => {
      // console.log({offer})
      peerConnection.setLocalDescription(offer)

      // console.log(location.pathname.substring(1))
      fetch(`http://localhost:8080/api/whep`, {
        method: 'POST',
        body: offer.sdp,
        headers: {
          Authorization: `Bearer ${location.pathname.substring(1)}`,
          'Content-Type': 'application/sdp'
        }
      }).then(r => {
        // const parsedLinkHeader = parseLinkHeader(r.headers.get('Link'))
        // console.log(r.headers.get("Link"))
        // setLayerEndpoint(`${window.location.protocol}//${parsedLinkHeader['urn:ietf:params:whep:ext:core:layer'].url}`)

        // console.log(window.location.protocol)

        // const evtSource = new EventSource(`${window.location.protocol}//${parsedLinkHeader['urn:ietf:params:whep:ext:core:server-sent-events'].url}`)
        // evtSource.onerror = err => evtSource.close();

        // evtSource.addEventListener("layers", event => {
        //   const parsed = JSON.parse(event.data)
        //   console.log({parsed})
        //   setVideoLayers(parsed['1']['layers'].map(l => l.encodingId))
        // })


        return r.text()
      }).then(answer => {
        peerConnection.setRemoteDescription({
          sdp: answer,
          type: 'answer'
        })
      })
    })

    return function cleanup() {
      peerConnection.close()
    }
  }, [location.pathname])

  return (
    <>
      <video
        ref={videoRef}
        autoPlay
        muted
        controls
        playsInline
        className={`bg-black w-full ${cinemaMode && "min-h-screen"}`}
      />

      {videoLayers.length >= 2 &&
        <select defaultValue="disabled" onChange={onLayerChange} className="appearance-none border w-full py-2 px-3 leading-tight focus:outline-none focus:shadow-outline bg-gray-700 border-gray-700 text-white rounded shadow-md placeholder-gray-200">
          <option value="disabled" disabled={true}>Choose Quality Level</option>
          {videoLayers.map(layer => {
            return <option key={layer} value={layer}>{layer}</option>
          })}
        </select>
      }
    </>
  )
}

export default PlayerPage
