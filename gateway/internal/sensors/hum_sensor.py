import socket
import time
import random

udp_host = "localhost" 
udp_port = 4638 

def humidity_sensor(): 
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)

    while True:
        humidity_value = random.randint(40, 90)
        # Send information if humidity exceeds 80
        if humidity_value > 80:
            message = f"{humidity_value}\r\n"
            sock.sendto(message.encode(), (udp_host, udp_port))
            print(f"Sent --> Humidity: {message}")

        # Send 'ALIVE' message every 3 seconds
        time.sleep(1)
        timestamp = int(time.time())
        if timestamp % 3 == 0:
            alive_message = "ALIVE\r\n"
            sock.sendto(alive_message.encode(), (udp_host, udp_port))
            print(f"Sent: {alive_message}")

if __name__ == "__main__":
    humidity_sensor()
