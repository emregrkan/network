import socket
import select
import sqlite3
import re
import json
import sys
from multiprocessing import Process


def db_conn():
    db = sqlite3.connect("server.db")
    db.row_factory = lambda c, r: dict([(col[0], r[idx]) for idx, col in enumerate(c.description)])
    return db


def ok(body):
    parsed = json.dumps(body)
    return f"HTTP/1.1 200 OK\r\nContent-Length: {len(parsed)}\r\nContent-Type: application/json\r\n\r\n{parsed}\r\n".encode()


def not_found():
    return "HTTP/1.1 404 Not Found\r\nContent-Length: 13\r\nContent-Type: text/plain\r\n\r\n404 Not Found\r\n".encode()


def parse_request(req):
    r = re.compile(r'^([A-Z]+)\s+([^?\s]+)')
    match = r.match(req)

    if match:
        method = match.group(1)  # Extract HTTP method (GET, POST, etc.)
        target = match.group(2)  # Extract the target URL
        return method, target
    else:
        return None


def sensor():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        with db_conn() as db:
            address_port = ("127.0.0.1", 14673)
            print(f'Listening sensors on {address_port}')
            s.bind(address_port)
            s.listen()
            conn, addr = s.accept()
            with conn:
                while True:
                    packet = conn.recv(1024)
                    if packet:
                        msg = packet.decode()
                        data = [tuple(data.split(':')) for data in msg.split()[1:]]
                        cursor = db.cursor()
                        table = "temperature" if msg[:4] == "TEMP" else "humidity"
                        cursor.executemany(f"INSERT INTO {table}(time, value) VALUES (?, ?)", data)
                        db.commit()


def http():
    with db_conn() as db:
        # Get socket file descriptor as a TCP socket using the IPv4 address family
        listener_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        # Set some modes on the socket, not required but it's nice for our uses
        listener_socket.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)

        address_port = ("127.0.0.1", 8080)
        # leserve address and port
        listener_socket.bind(address_port)
        # listen for connections, a maximum of 1
        listener_socket.listen(1)
        print("Server listening @ 127.0.0.1:8080")

        # loop indefinitely to continuously check for new connections
        while True:
            # Poll the socket to see if there are any newly written data, note excess data dumped to "_" variables
            read_ready_sockets, _, _ = select.select(
                [listener_socket],  # list of items we want to check for read-readiness (just our socket)
                [],  # list of items we want to check for write-readiness (not interested)
                [],  # list of items we want to check for "exceptional" conditions (also not interested)
                0  # timeout of 0 seconds, makes the method call non-blocking
            )
            # if a value was returned here then we have a connection to read from
            if read_ready_sockets:
                # select.select() returns a list of readable objects, so we'll iterate, but we only expect a single item
                for ready_socket in read_ready_sockets:
                    # accept the connection from the client and get its socket object and address
                    client_socket, client_address = ready_socket.accept()

                    raw = client_socket.recv(4096)
                    method, path = parse_request(raw.decode())

                    if method == 'GET' and (path == '/temperature' or path == '/humidity'):
                        # Send a response to the client, notice it is a byte string
                        cursor = db.cursor()
                        result = cursor.execute(f"SELECT * FROM {path[1:]} ORDER BY time DESC")
                        client_socket.sendall(ok(result.fetchall()))
                    else:
                        client_socket.sendall(not_found())

                    try:
                        # close the connection
                        client_socket.close()
                    except OSError:
                        # client disconnected first, nothing to do
                        pass


if __name__ == '__main__':
    s = Process(target=sensor, daemon=True)
    try:
        s.start()
        http()
    except KeyboardInterrupt:
        s.terminate()
        sys.exit(0)
