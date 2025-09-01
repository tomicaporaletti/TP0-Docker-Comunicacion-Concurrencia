import socket
import logging
import sys



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
        logging.info(f"action: signal | type: {signum} | result: received")
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
            msg     = self.__recv_until_new_line__(client_sock)
            addr    = client_sock.getpeername()
            logging.info(
                f'action: receive_message | result: success | ip: {addr[0]} | msg: {msg}'
            )
            self.__send_line__(client_sock, msg)
        except OSError as e:
            logging.error(f"action: handle_client | result: fail | error: {e}") # Cambie el nombre de action para que sea mas descriptivo
        finally:
            client_sock.close()

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