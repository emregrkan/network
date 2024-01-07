import socket
import select
import sqlite3
import re
import json
import sys
from multiprocessing import Process
from datetime import datetime


def convert_to_datetime(timestamp):
    return datetime.fromtimestamp(timestamp)

def db_conn():
    db = sqlite3.connect("server.db")
    db.row_factory = lambda c, r: dict([(col[0], r[idx]) for idx, col in enumerate(c.description)])
    return db


def ok(body, content_type="text/html"):
    return f"HTTP/1.1 200 OK\r\nContent-Length: {len(body)}\r\nContent-Type: {content_type}\r\n\r\n{body}\r\n".encode()

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
        addr = ("127.0.0.1", 8080)
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        sock.bind(addr)
        sock.listen(1)
        print(f"HTTP Server running at {addr}")

        while True:
            ready, _, _ = select.select([sock], [], [], 0)

            if ready:
                for ready_sock in ready:
                    client_socket, _ = ready_sock.accept()
                    raw = client_socket.recv(4096)
                    method, path = parse_request(raw.decode())

                    if method == 'GET' and (path == '/temperature' or path == '/humidity'):
                        cursor = db.cursor()
                        result = cursor.execute(f"SELECT * FROM {path[1:]} ORDER BY time DESC")
                        rows = result.fetchall()
                        table_html = "<table border='1'><tr><th>Time</th><th>Value</th></tr>"

                        for row in rows:
                            datetime_obj = convert_to_datetime(row['time'])
                            table_html += f"<tr><td>{datetime_obj}</td><td>{row['value']}</td></tr>"

                        table_html += "</table>"
                        client_socket.sendall(ok(table_html, content_type="text/html"))
                    else:
                        client_socket.sendall(not_found())

                    try:
                        client_socket.close()
                    except OSError:
                        pass


if __name__ == '__main__':
    s = Process(target=sensor, daemon=True)
    try:
        s.start()
        http()
    except KeyboardInterrupt:
        s.terminate()
        sys.exit(0)
