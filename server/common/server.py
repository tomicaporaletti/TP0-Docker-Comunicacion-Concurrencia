import socket
import logging
from . import utils as u
from . import protocol as p

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


    def __handle_client_connection(self, client_sock):
        try:
            data = p.recv_message(client_sock)
            resp = self.__process_msg(data)
            p.send_confirmation(client_sock, resp.get("result") == "success")
        except Exception as e:
            logging.error(f"action: handle_client | result: fail | error: {e}")
        finally:
            client_sock.close()


    def __process_msg(self, data: dict) -> dict:
        """Procesa un mensaje JSON y delega según el type."""
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