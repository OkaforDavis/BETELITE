import re

with open('mobile/index.html', 'r', encoding='utf-8') as f:
    content = f.read()

# Define the start and end markers
start_marker = r'// ══════════════════════════════════════════════════════\n    //  WEBRTC LIVE STREAMING'
end_marker = r'toast\(\'❌ Error joining stream\'\);\n      }\n    }'

pattern = re.compile(start_marker + r'.*?' + end_marker, re.DOTALL)

replacement = """// ══════════════════════════════════════════════════════
    //  LIVEKIT STREAMING & WEBSOCKET CHAT
    // ══════════════════════════════════════════════════════
    let currentRoom = null;
    let chatWs = null;

    function connectChat() {
      if (chatWs) return;
      const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      chatWs = new WebSocket(`${wsProtocol}//${window.location.host}/ws`);
      
      chatWs.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          if (msg.type === 'chat_message' && msg.matchId === currentStreamMatchId) {
            appendChatMessage(msg);
          }
        } catch(e) {}
      };
      
      chatWs.onclose = () => {
        chatWs = null;
        setTimeout(connectChat, 3000); // Reconnect logic
      };
    }
    
    // Connect chat on load
    connectChat();

    function sendChatMessage() {
      const input = document.getElementById('chat-input');
      if (!input || !input.value.trim() || !chatWs) return;
      
      chatWs.send(JSON.stringify({
        type: 'chat_message',
        matchId: currentStreamMatchId,
        username: S.user?.displayName || 'Player',
        avatar: S.user?.photoURL || '',
        text: input.value.trim()
      }));
      input.value = '';
    }

    function appendChatMessage(msg) {
      const chatBox = document.getElementById('chat-messages');
      if (!chatBox) return;
      
      const div = document.createElement('div');
      div.style.cssText = 'display:flex; align-items:flex-start; gap:8px; margin-bottom:8px; background:rgba(0,0,0,0.5); padding:8px; border-radius:8px;';
      div.innerHTML = `
        <img src="${msg.avatar || 'crestarena-icon.png'}" style="width:24px; height:24px; border-radius:50%; object-fit:cover;">
        <div>
          <span style="font-size:12px; font-weight:bold; color:var(--gold);">${msg.username}</span><br>
          <span style="font-size:13px; color:#fff;">${msg.text}</span>
        </div>
      `;
      chatBox.appendChild(div);
      chatBox.scrollTop = chatBox.scrollHeight;
    }

    async function startWebRTCStream(matchId) {
      currentStreamMatchId = matchId;
      document.getElementById('stream-overlay').innerHTML = 'Requesting Screen...';
      
      try {
        const stream = await navigator.mediaDevices.getDisplayMedia({ video: true, audio: true });
        await connectToLiveKit(matchId, true, stream);
      } catch (err) {
        alert('Screen sharing failed. Trying camera fallback...');
        try {
          const cam = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
          await connectToLiveKit(matchId, true, cam);
        } catch (e) {
          alert('Could not start any stream.');
        }
      }
    }

    async function joinWebRTCStream(matchId) {
      currentStreamMatchId = matchId;
      document.getElementById('stream-overlay').innerHTML = 'Connecting...';
      await connectToLiveKit(matchId, false, null);
    }

    async function connectToLiveKit(matchId, isHost, localMediaStream) {
      try {
        const res = await fetch(BACKEND_URL + '/api/stream/token', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ roomName: matchId, identity: S.user?.displayName || 'Viewer_' + Math.floor(Math.random()*100), isHost })
        });
        const data = await res.json();
        
        if (!data.token) {
          alert('Failed to get stream token. Check LiveKit keys.');
          return;
        }

        const room = new LivekitClient.Room();
        currentRoom = room;

        room.on(LivekitClient.RoomEvent.TrackSubscribed, (track, publication, participant) => {
          if (track.kind === 'video') {
            const videoEl = document.getElementById('webrtc-video');
            track.attach(videoEl);
            document.getElementById('stream-overlay').style.display = 'none';
          }
        });

        await room.connect('wss://betelite-38umojt1.livekit.cloud', data.token);

        if (isHost && localMediaStream) {
          const videoTrack = localMediaStream.getVideoTracks()[0];
          await room.localParticipant.publishTrack(videoTrack, { name: 'screen' });
          const videoEl = document.getElementById('webrtc-video');
          videoEl.srcObject = localMediaStream;
          document.getElementById('stream-overlay').style.display = 'none';
          toast('Stream is LIVE globally!');
        } else if (!isHost) {
          toast('Connected to stream!');
        }

        // Add Chat UI overlay
        addChatOverlay();

      } catch (e) {
        console.error(e);
        alert('Streaming error: ' + e.message);
      }
    }

    function addChatOverlay() {
      let existing = document.getElementById('stream-chat-overlay');
      if (existing) return;
      
      const parent = document.getElementById('webrtc-video').parentElement;
      const overlay = document.createElement('div');
      overlay.id = 'stream-chat-overlay';
      overlay.style.cssText = 'position:absolute; bottom:40px; left:10px; right:10px; height:150px; display:flex; flex-direction:column; justify-content:flex-end; pointer-events:none; z-index:10;';
      
      const msgBox = document.createElement('div');
      msgBox.id = 'chat-messages';
      msgBox.style.cssText = 'overflow-y:auto; margin-bottom:10px; pointer-events:auto; display:flex; flex-direction:column; max-height:100px;';
      
      const inputRow = document.createElement('div');
      inputRow.style.cssText = 'display:flex; gap:8px; pointer-events:auto;';
      inputRow.innerHTML = `
        <input type="text" id="chat-input" placeholder="Say something..." style="flex:1; border-radius:20px; border:none; padding:8px 12px; background:rgba(0,0,0,0.7); color:#fff; font-size:14px;">
        <button onclick="sendChatMessage()" style="background:var(--gold); border:none; border-radius:50%; width:36px; height:36px; color:#000; font-weight:bold; cursor:pointer;">➔</button>
      `;
      
      overlay.appendChild(msgBox);
      overlay.appendChild(inputRow);
      parent.appendChild(overlay);
    }
    
    function toggleStreamFullscreen() {
      const container = document.getElementById('stream-container');
      if (!container) return;
      
      if (container.dataset.fullscreen === 'true') {
        container.style.cssText = 'background:#000; border-radius:8px; width:100%; height:200px; display:flex; flex-direction:column; justify-content:center; align-items:center; border:1px solid var(--border2); position:relative; overflow:hidden;';
        container.dataset.fullscreen = 'false';
      } else {
        container.style.cssText = 'position:fixed; top:0; left:0; right:0; margin:0 auto; max-width:430px; width:100%; height:100vh; height:100dvh; background:#000; z-index:500; display:flex; flex-direction:column; justify-content:center; align-items:center;';
        container.dataset.fullscreen = 'true';
      }
    }"""

new_content, count = pattern.subn(replacement, content)

if count > 0:
    with open('mobile/index.html', 'w', encoding='utf-8') as f:
        f.write(new_content)
    print('Replacement successful.')
else:
    print('Pattern not found.')
