import socket
import logging
from . import utils as u
from . import protocol as p

class Server:
    def __init__(self, port, listen_backlog, total_clients):
        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        # === Socket config === #
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        self._server_socket.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        self._server_socket.settimeout(1.0)
        self._shutdown = False
        # ===================== #
        self._total_clients     = total_clients
        self._notificaciones    = 0
        self._sorteo_realizado  = False
        self._pending_queries   = {}  
        self._ganadores         = {}  

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
            resp = self.__process_msg(data, client_sock)
            if resp["type"] == "confirmation":
                p.send_confirmation(client_sock, resp.get("result") == "success")
                client_sock.close()
            elif resp["type"] == "winners":
                p.send_winners(client_sock, resp["winners"])
                logging.info(
                    f"action: consulta_ganadores | result: success | agency: {data['agency']} | cant_ganadores: {len(resp['winners'])}"
                )
                client_sock.close()
            elif resp["type"] == "pending":
                # no cierro el socket -> se queda abierto en _pending_queries
                return
            else:
                p.send_confirmation(client_sock, False)
                client_sock.close()
        except Exception as e:
            logging.error(f"action: handle_client | result: fail | error: {e}")
            client_sock.close()


    def __process_msg(self, data: dict, client_sock) -> dict:
        """Procesa un mensaje y delega según el type."""
        msg_type = data.get("type")
        if msg_type == "bet":
            return self._handle_bet(data)
        elif msg_type == "batch":
            return self._handle_batch(data)
        elif msg_type == "notify_end":
            return self._handle_notify(data)
        elif msg_type == "query_winners":
            return self._handle_query(data, client_sock)
        else:
            logging.error(f"action: process_msg | result: fail | error: unknown_type | type: {msg_type}")
            return {"type": "error", "reason": "unknown_type"}


    # ================================================ #
    # ====== HANDLERS PARA CADA TIPO DE MENSAJE ====== #
    # ================================================ #

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
        
    def _handle_batch(self, data: dict) -> dict:
        """Procesa un mensaje con type=batch."""
        try:
            bets = self._parse_batch(data["bets"])
            u.store_bets(bets)
            logging.info(
                f"action: apuesta_recibida | result: success | cantidad: {len(bets)}"
            )
            return {"type": "confirmation", "result": "success"}
        except Exception as e:
            cantidad = len(bets) if "bets" in locals() else 0
            logging.error(f"action: apuesta_recibida | result: fail | cantidad: {cantidad}")
            return {"type": "error", "reason": "invalid_bet"}
        
    def _handle_notify(self, data: dict) -> dict:
        """Procesa un mensaje con type=notify_end"""
        self._notificaciones += 1
        logging.info(f"action: notify_end | result: success | agency: {data['agency']} | total: {self._notificaciones}")
        if self._notificaciones == self._total_clients:
            logging.info("action: all_clients_notified | result: success")
            self._run_sorteo()
        return {"type": "confirmation", "result": "success"}

    def _handle_query(self, data: dict, client_sock) -> dict:
        """Procesa un mensaje con type=query_winners"""
        agency = data["agency"]
        if not self._sorteo_realizado:
            # guardo este socket para responder más tarde
            self._pending_queries.setdefault(agency, []).append(client_sock)
            logging.info(f"action: consulta_ganadores | result: in_progress | agency: {agency}")
            return {"type": "pending"}
        winners = self._ganadores.get(agency, [])
        return {"type": "winners", "winners": winners}

    # ================================================ #
    # ======  PARSERS PARA CADA TIPO DE MENSAJE ====== #
    # ================================================ #

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
    
    def _run_sorteo(self):
        """Calcula ganadores por agencia."""
        all_bets = list(u.load_bets())
        self._ganadores = {}
        for bet in all_bets:
            if u.has_won(bet):
                self._ganadores.setdefault(bet.agency, []).append(bet.document)
        self._sorteo_realizado = True
        logging.info("action: sorteo | result: success")

        for agency, sockets in self._pending_queries.items():
            winners = self._ganadores.get(agency, [])
            for s in sockets:
                try:
                    p.send_winners(s, winners)
                    logging.info(f"action: consulta_ganadores | result: success | agency: {agency} | cant_ganadores: {len(winners)}")
                except Exception as e:
                    logging.error(f"action: consulta_ganadores | result: fail | agency: {agency} | error: {e}")
                finally:
                    s.close()
        self._pending_queries.clear()

    def _parse_batch(self, data:list[dict]) -> list[u.Bet]:
        """Convierte un dict a una lista de Bets validadas."""
        bets: list[u.Bet] = []
        for bet in data:
            bets.append(self._parse_bet(bet))
        return bets

    def __accept_new_connection(self):
        """
        Accept new connections

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned
        """
        c, addr = self._server_socket.accept()
        c.settimeout(None) 
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