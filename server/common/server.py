import socket
import logging
import json
import utils as u

class Server:
    def __init__(self, port, listen_backlog):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self._server_socket.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        self._server_socket.settimeout(1.0)
        self._shutdown = False

    def run(self):
        """
        Dummy Server loop

        Server that accept a new connections and establishes a
        communication with a client. After client with communucation
        finishes, servers starts to accept new connections again
        """
        logging.info("action: server_start | result: success")
        try:
            while not self._shutdown:
                try:
                    client_sock = self.__accept_new_connection()
                except socket.timeout:
                    continue
                except OSError:
                    break

                self.__handle_client_connection(client_sock)
        finally:
            self._graceful_shut()


    def request_shutdown(self, signum=None, frame=None):
        """Handler para SIGTERM: marca shutdown."""
        sig_name = "SIGTERM" if signum == 15 else str(signum)
        logging.info(f"action: signal | result: success | signal: {sig_name}")
        self._shutdown = True


    def __recv_until_new_line__(self, client_sock) -> str:
        """
        Lee hasta encontrar el salto de linea. 
        Devuelve mensaje sin el "\n".
        """
        client_sock.settimeout(15)
        buf = bytearray()
        while True:
            chunk = client_sock.recv(1024)
            if not chunk:
                raise ConnectionError("peer closed before newline")
            buf += chunk
            nl = buf.find(b"\n")
            if nl != -1:
                line = bytes(buf[:nl])
                return line.decode("utf-8")


    def __send_line__(self, client_sock, msg: str) -> None:
        """
        Envia msg echo al cliente asegurando entrega completa.
        """
        echo        = (msg + "\n").encode("utf-8")
        total_sent  = 0

        while total_sent < len(echo):
            sent = client_sock.send(echo[total_sent:])
            if sent == 0:
                raise ConnectionError("socket closed while sending")
            total_sent += sent


    def __handle_client_connection(self, client_sock):
        """
        Read message from a specific client socket and closes the socket

        If a problem arises in the communication with the client, the
        client socket will also be closed
        """
        try:
            msg         = self.__recv_until_new_line__(client_sock)
            addr        = client_sock.getpeername()
            resp        = self.__process_msg(msg)
            resp_json   = json.dumps(resp)
            self.__send_line__(client_sock, resp_json)
        except OSError as e:
            logging.error(f"action: handle_client | result: fail | error: {e}") # Cambie el nombre de action para que sea mas descriptivo
        finally:
            client_sock.close()


    def __process_msg(self, msg: str) -> dict:
        """Procesa un mensaje JSON y delega según el type."""
        try:
            data = json.loads(msg)
        except json.JSONDecodeError as e:
            logging.error(f"action: process_msg | result: fail | error: invalid_json | detail: {e}")
            return {"type": "error", "reason": "invalid_json"}

        msg_type = data.get("type")
        if msg_type == "bet":
            return self._handle_bet(data)
        else:
            logging.error(f"action: process_msg | result: fail | error: unknown_type | type: {msg_type}")
            return {"type": "error", "reason": "unknown_type"}


    def _handle_bet(self, data: dict) -> dict:
        """Procesa un mensaje con type=bet."""
        try:
            bet = self._parse_bet(data)
            u.store_bets([bet])  # store_bets espera lista
            logging.info(
                f"action: apuesta_almacenada | result: success | dni: {bet.document} | numero: {bet.number}"
            )
            return {"type": "confirmation", "result": "success"}
        except Exception as e:
            logging.error(f"action: handle_bet | result: fail | error: {e}")
            return {"type": "error", "reason": "invalid_bet"}


    def _parse_bet(self, data: dict) -> u.Bet:
        """Convierte un dict a una instancia de Bet validada."""
        return u.Bet(
            agency      =  data["agency"],
            first_name  =  data["first_name"],
            last_name   =  data["last_name"],
            document    =  data["document"],
            birthdate   =  data["birthdate"],
            number      =  data["number"],
        )


    def __accept_new_connection(self):
        """
        Accept new connections

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned
        """
        c, addr = self._server_socket.accept()
        logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
        return c


    def _graceful_shut(self):
        """
        Cierra el listening socket y loguea el shutdown.
        """
        try:
            self._server_socket.close()
            logging.info("action: server_socket_close | result: success")
        except OSError as e:
            logging.error(f"action: server_socket_close | result: fail | error: {e}")
        logging.info("action: exit | result: success")   # <--- NECESARIO PARA LOS TESTS