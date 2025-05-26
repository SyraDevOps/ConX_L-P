from fastapi import FastAPI, WebSocket, WebSocketDisconnect, HTTPException
from fastapi.responses import JSONResponse
from typing import Dict, Any
from pydantic import BaseModel

app = FastAPI()

class PeerInfo(BaseModel):
    peer_id: str
    role: str  # "client" ou "server"

# Estrutura para armazenar peers conectados
class PeerConnection:
    def __init__(self, websocket: WebSocket, role: str):
        self.websocket = websocket
        self.role = role
        self.partner: str = None  # peer_id do parceiro relay

peers: Dict[str, PeerConnection] = {}

@app.websocket("/ws/{peer_id}/{role}")
async def websocket_endpoint(websocket: WebSocket, peer_id: str, role: str):
    await websocket.accept()
    peers[peer_id] = PeerConnection(websocket, role)
    try:
        while True:
            data = await websocket.receive_text()
            # Se estiver em modo relay, repassa para o parceiro
            partner_id = peers[peer_id].partner
            if partner_id and partner_id in peers:
                await peers[partner_id].websocket.send_text(data)
    except WebSocketDisconnect:
        # Remove peer e desfaz relay se necessário
        partner_id = peers[peer_id].partner
        if partner_id and partner_id in peers:
            peers[partner_id].partner = None
        peers.pop(peer_id, None)

@app.post("/relay/connect")
async def connect_relay(peer_a: str, peer_b: str):
    if peer_a not in peers or peer_b not in peers:
        raise HTTPException(status_code=404, detail="Peer não conectado")
    # Liga os peers para relay
    peers[peer_a].partner = peer_b
    peers[peer_b].partner = peer_a
    return {"status": "relay established", "peers": [peer_a, peer_b]}

@app.post("/relay/disconnect")
async def disconnect_relay(peer_id: str):
    if peer_id not in peers:
        raise HTTPException(status_code=404, detail="Peer não conectado")
    partner_id = peers[peer_id].partner
    if partner_id and partner_id in peers:
        peers[partner_id].partner = None
    peers[peer_id].partner = None
    return {"status": "relay disconnected", "peer": peer_id}

@app.get("/peers")
def list_peers():
    return [
        {
            "peer_id": pid,
            "role": conn.role,
            "relay_partner": conn.partner
        }
        for pid, conn in peers.items()
    ]