import subprocess
import sys

"""
subprocess.check_output
Es una funcion de Python que ejecuta un 
comando externo (como si lo escribieras en la terminal) 
y te devuelve lo que imprime a stdout.
Si el comando falla (código de salida ≠ 0), 
lanza subprocess.CalledProcessError.
"""

def get_port():
    """
    Lee el puerto desde el archivo config.ini
    """
    port = "12345"
    try:
        with open("server/config.ini") as f:
            for line in f:
                if line.strip().startswith("SERVER_PORT"):
                    port = line.split("=")[1].strip()    
    except FileNotFoundError:
        pass
    return port

def lookup_docker_net():
    """
    Busca la red definida en el archivo DockerCompose.
    Comando equivalente en la Shell
    docker network ls --format '{{.Name}}' | grep -E '_testing_net$' | head -n1
    """
    try:
        out = subprocess.check_output(["docker","network","ls","--format","{{.Name}}"], text=True)
        netname = next((n for n in out.splitlines() if n.endswith("_testing_net")), "")

    except subprocess.CalledProcessError:
        netname = ""

    if not netname:
        print("action: test_echo_server | result: fail")
        sys.exit(1)

    return netname

def validate_server(netname, port, msg):
    """
    Valida el correcto funcionamiento del servidor, enviando un mensaje
    y esperando recibir el mismo de vuelta.
    Comando equivalente en la Shell
    docker run --rm --network <red> busybox sh -c "printf '%s\n' 'Hola' | nc -w 3 server 12345"
    """
    try:
        resp = subprocess.check_output(
            ["docker","run","--rm","--network",netname,"-e",f"MSG={msg}","-e",f"PORT={port}",
            "busybox","sh","-c","printf '%s\n' \"$MSG\" | nc -w 3 server \"$PORT\""],
            text=True
        ).strip()

    except subprocess.CalledProcessError:
        resp = ""

    if resp == msg:
        print("action: test_echo_server | result: success")
        sys.exit(0)
    else:
        print("action: test_echo_server | result: fail")
        sys.exit(1)


def main():
    # Mensaje a enviar: primer argumento o "Hola"
    msg = sys.argv[1] if len(sys.argv) > 1 else "Hola"

    # Leer puerto desde config.ini
    port = get_port()

    # Buscar la red de DockerCompose
    netname = lookup_docker_net()

    # Validar correcto funcionamiento del servidor
    validate_server(netname, port, msg)

if __name__ == "__main__":
    main()
