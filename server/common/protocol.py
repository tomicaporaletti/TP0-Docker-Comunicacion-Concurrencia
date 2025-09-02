import socket

# --- Constantes del protocolo ---
MSG_TYPE_BET        = 1
MSG_TYPE_CONFIRM    = 100


# == Helpers de bajo nivel ==

def recv_exact(sock: socket.socket, n: int) -> bytes:
    """Lee exactamente n bytes del socket o lanza error."""
    buf = b""
    while len(buf) < n:
        chunk = sock.recv(n - len(buf))
        if not chunk:
            raise ConnectionError("socket closed mid-read")
        buf += chunk
    return buf


def recv_string(sock: socket.socket) -> str:
    """Lee string con protocolo [1 byte length][bytes]."""
    length  = recv_exact(sock, 1)[0]
    data    = recv_exact(sock, length)
    return data.decode("utf-8")


def send_all(sock: socket.socket, data: bytes) -> None:
    """Envia todos los bytes evitando short-write."""
    total = 0
    while total < len(data):
        n = sock.send(data[total:])
        if n == 0:
            raise ConnectionError("socket closed while sending")
        total += n


# == Decodificación de mensajes ==

def recv_message(sock: socket.socket) -> dict:
    """
    Decodifica un mensaje del cliente.
    Protocolo:
      [1 byte type]
      [4 bytes agency]
      [dni str] [first str] [last str] [birth str]
      [4 bytes number]

    Protocolo usa Big Endiann
    """
    msg_type = recv_exact(sock, 1)[0]
    if msg_type != MSG_TYPE_BET:
        raise ValueError(f"Unknown message type {msg_type}")

    agency      = int.from_bytes(recv_exact(sock, 4), "big")
    document    = recv_string(sock)
    first       = recv_string(sock)
    last        = recv_string(sock)
    birth       = recv_string(sock)
    number      = int.from_bytes(recv_exact(sock, 4), "big")

    return {
        "type"      : "bet",
        "agency"    : agency,
        "document"  : document,
        "first_name": first,
        "last_name" : last,
        "birthdate" : birth,
        "number"    : number,
    }


# == Serialización de respuestas ==

def send_confirmation(sock: socket.socket, success: bool) -> None:
    """
    Envía respuesta:
      [1 byte type=100]
      [1 byte result=1 exito, 0 fallo]
    """
    data = bytes([MSG_TYPE_CONFIRM, 1 if success else 0])
    send_all(sock, data)
