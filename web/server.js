import * as mediasoupClient from "mediasoup-client";
const { parseLinkHeader } = require('@web3-storage/parse-link-header');
const { RTCPeerConnection, RTCSessionDescription, RTCRtpReceiver } = require('wrtc');
const EventSource = require('eventsource')
const { RTCAudioSink, RTCVideoSink } = require('wrtc').nonstandard;
const { PassThrough } = require('node:stream')

async function createPeerConnection() {
  const configuration = { iceServers: [{ urls: 'stun:stun.l.google.com:19302' }] };
  const peerConnection = new RTCPeerConnection({});

  // Handle incoming data, such as video and audio streams
  peerConnection.ontrack = (event) => {
     // Create encoded stream and transformer
      const encodedStream = event.receiver.createEncodedStreams();
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
 
    encodedStream.readable.pipeThrough(transformer)
    .pipeTo(encodedStream.writable);
      

    console.log('Received remote stream:', event.streams[0]);
    // You can handle the incoming media stream here
  };

  const audioTransceiver = peerConnection.addTransceiver('audio')
  const videoTransceiver = peerConnection.addTransceiver('video')
  const videoCapabilities = RTCRtpReceiver.getCapabilities('video')
  const audioCapabilities = RTCRtpReceiver.getCapabilities('audio')

  console.log(videoCapabilities)

  videoTransceiver.setCodecPreferences(videoCapabilities.codecs)
  audioTransceiver.setCodecPreferences(audioCapabilities.codecs)


  const audioSink = new RTCAudioSink(audioTransceiver.receiver.track);
  const videoSink = new RTCVideoSink(videoTransceiver.receiver.track);

  const streams = [];

//   videoSink.addEventListener('frame', ({ frame: { width, height, data }}) => {
//     const size = width + 'x' + height;
//     if (!streams[0] || (streams[0] && streams[0].size !== size)) {
//     //   UID++;

//       const stream = {
//         // recordPath: './recording-' + size + '-' + UID + '.mp4',
//         size,
//         video: new PassThrough(),
//         audio: new PassThrough()
//       };

//       const onAudioData = ({ samples: { buffer } }) => {
//         if (!stream.end) {
//           stream.audio.push(Buffer.from(buffer));
//         }
//       };

//       audioSink.addEventListener('data', onAudioData);

//       stream.audio.on('end', () => {
//         audioSink.removeEventListener('data', onAudioData);
//       });

//       streams.unshift(stream);

//       streams.forEach(item=>{
//         if (item !== stream && !item.end) {
//           item.end = true;
//           if (item.audio) {
//             item.audio.end();
//           }
//           item.video.end();
//         }
//       })
//     }

//     streams[0].video.push(Buffer.from(data));
// });

  // Handle ICE candidate events
  peerConnection.onicecandidate = (event) => {
    if (event.candidate) {
      // Send the ICE candidate to the other peer (via signaling server)
    //   console.log('ICE Candidate:', event.candidate);
    }
    }
    
//   };

  // Create an offer to initiate the connection
  const offer = await peerConnection.createOffer();
  await peerConnection.setLocalDescription(offer);

  // Send the offer to the other peer (via signaling server)
  console.log('Offer:', offer);

  fetch(`http://localhost:8080/api/whep`, {
    method: 'POST',
    body: offer.sdp,
    headers: {
      Authorization: `Bearer StreamTest`,
      'Content-Type': 'application/sdp'
    }
  }).then(r => {
    // const parsedLinkHeader = parseLinkHeader(r.headers.get('Link'))
    // console.log(parsedLinkHeader)
    // // setLayerEndpoint(`${window.location.protocol}//${parsedLinkHeader['urn:ietf:params:whep:ext:core:layer'].url}`)

    // // console.log(window.location.protocol)

    // const evtSource = new EventSource(`http://${parsedLinkHeader['urn:ietf:params:whep:ext:core:server-sent-events'].url}`)
    // evtSource.onerror = err => evtSource.close();

    // evtSource.addEventListener("layers", event => {
    //   const parsed = JSON.parse(event.data)
    // //   setVideoLayers(parsed['1']['layers'].map(l => l.encodingId))
    // })


    return r.text()
  }).then(answer => {
    console.log({answer})
    peerConnection.setRemoteDescription({
      sdp: answer,
      type: 'answer'
    })
  })

  // Handle the answer from the other peer (received via signaling)
  // Set the remote description with the answer
  // const answer = ... // Received answer from the other peer
  // await peerConnection.setRemoteDescription(new RTCSessionDescription(answer));
}

createPeerConnection();
