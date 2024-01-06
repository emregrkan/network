# echo-client.py

import sys
import socket
import time
import random

HOST = "localhost"  # The server's hostname or IP address
PORT = 8367  # The port used by the server

try:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.connect((HOST, PORT))
        while True:
            msg = f"TEMP {int(time.time())}:{random.randint(0, 40)}\r\n"    
            s.send(msg.encode())
            time.sleep(1)
except KeyboardInterrupt:
    sys.exit(0)

