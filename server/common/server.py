import socket
import logging


class Server:
    def __init__(self, port, listen_backlog):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)

    def run(self):
        """
        Dummy Server loop

        Server that accept a new connections and establishes a
        communication with a client. After client with communucation
        finishes, servers starts to accept new connections again
        """

        # TODO: Modify this program to handle signal to graceful shutdown
        # the server
        while True:
            client_sock = self.__accept_new_connection()
            self.__handle_client_connection(client_sock)

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

        # Connection arrived
        logging.info('action: accept_connections | result: in_progress')
        c, addr = self._server_socket.accept()
        logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
        return c
