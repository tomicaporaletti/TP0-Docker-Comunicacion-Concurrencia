import socket

# --- Constantes del protocolo ---
MSG_TYPE_BET        = 1
MSG_TYPE_BATCH      = 2
MSG_TYPE_NOTIFY_END = 3
MSG_TYPE_QUERY_WIN  = 4
MSG_TYPE_CONFIRM    = 100
MSG_TYPE_WINNERS    = 101


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

def recv_one_bet(sock: socket.socket) -> dict:
    """
    Decodifica una apuesta.
    Protocolo:
      [4 bytes agency]
      [dni str] [first str] [last str] [birth str]
      [4 bytes number]

    Protocolo usa Big Endiann
    """
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

def recv_batch(sock: socket.socket) -> dict:
    """
    Decodifica cuantas apuestas hay en el batch.
    Protocolo:
      [2 bytes total bets]

    Protocolo usa Big Endiann
    """
    tot_bets            = int.from_bytes(recv_exact(sock, 2), "big")
    bets: list[dict]    = []
    for _ in range(tot_bets):
        bets.append(recv_one_bet(sock))
    return {
        "type"      : "batch",
        "bets"      : bets,
    }


def recv_notify(sock: socket.socket) -> dict:
    """
    Protocolo:
      [1 byte type=3]
      [4 bytes agency].
    """
    agency = int.from_bytes(recv_exact(sock, 4), "big")
    return {"type": "notify_end", "agency": agency}


def recv_query(sock: socket.socket) -> dict:
    """
    Decodifica la consulta de la lista de ganadores.
    Protocolo:
      [1 byte type=4]
      [4 bytes agency]
    """
    agency = int.from_bytes(recv_exact(sock, 4), "big")
    return {"type": "query_winners", "agency": agency}


def recv_message(sock: socket.socket) -> dict:
    """
    Decodifica el tipo de mensaje que llego al servidor.
    Protocolo:
      [1 byte type]

    Protocolo usa Big Endiann
    """
    msg_type = recv_exact(sock, 1)[0]
    if    msg_type == MSG_TYPE_BET:
        return recv_one_bet(sock)
    elif  msg_type == MSG_TYPE_BATCH:
        return recv_batch(sock)
    elif  msg_type == MSG_TYPE_NOTIFY_END:
        return recv_notify(sock)
    elif  msg_type == MSG_TYPE_QUERY_WIN: 
        return recv_query(sock)
    else:
        raise ValueError(f"Unknown message type {msg_type}")

# == Serialización de respuestas ==

def send_confirmation(sock: socket.socket, success: bool) -> None:
    """
    Envía respuesta:
      [1 byte type=100]
      [1 byte result=1 exito, 0 fallo]
    """
    data = bytes([MSG_TYPE_CONFIRM, 1 if success else 0])
    send_all(sock, data)

def send_winners(sock: socket.socket, winners: list[str]) -> None:
    """
    Protocolo:
      [1 byte type=101]
      [2 bytes cantidad]
      [1 byte type=101]
      [2 bytes cantidad de ganadores]
      [dni_1_length (1 byte)][dni_1 (N bytes)]
      ...
      [dni_n_length (1 byte)][dni_n (N bytes)]
    """
    out = bytearray()
    out.append(MSG_TYPE_WINNERS)
    out += len(winners).to_bytes(2, "big")
    for dni in winners:
        part = dni.encode("utf-8")
        if len(part) > 255:
            raise ValueError("DNI demasiado largo")
        out.append(len(part))
        out += part
    send_all(sock, bytes(out))