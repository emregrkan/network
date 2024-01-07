import socket
import time
import random

UDP_HOST = "localhost"
UDP_PORT = 4638
SOCKET_TIMEOUT = 5  # Set the timeout value in seconds

def send_handshake_request(sock):
    try:
        request_msg = "REQUEST TRANSMISSION\r\n"
        sock.sendto(request_msg.encode(), (UDP_HOST, UDP_PORT))
        
        # Set a timeout for receiving the response
        sock.settimeout(SOCKET_TIMEOUT)
        
        response_code, _ = sock.recvfrom(1024)
        response_code = int(response_code.decode())

        if response_code == 200:
            print("Handshake successful.")
        else:
            print(f"Handshake failed with status code {response_code}.")

        return response_code

    except socket.timeout:
        print("Timeout during handshake. Check the connection.")
        return -1

    except Exception as e:
        print(f"Error during handshake: {e}")
        return -1

def send_humidity_data(sock):
    try:
        with sock:
            response_code = send_handshake_request(sock)

            if response_code == 200:
                while True:
                    humidity_value = random.randint(40, 90)

                    # Send information if humidity exceeds 80
                    if humidity_value > 80:
                        message = f"HUMID {int(time.time())}:{humidity_value}\r\n"
                        sock.sendto(message.encode(), (UDP_HOST, UDP_PORT))
                        print(f"Sent --> {message}")

                    # Send 'ALIVE' message every 3 seconds
                    time.sleep(1)
                    timestamp = int(time.time())
                    if timestamp % 3 == 0:
                        alive_message = "ALIVE\r\n"
                        sock.sendto(alive_message.encode(), (UDP_HOST, UDP_PORT))
                        print(f"Sent: {alive_message}")

            else:
                print(f"Bad response from the gateway. Handshake failed with status code {response_code}.")

    except socket.timeout:
        print("Timeout during data transmission. Check the connection.")

    except Exception as e:
        print(f"Internal server error: {e}")

if __name__ == "__main__":
    try:
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as s:
            s.settimeout(SOCKET_TIMEOUT)  # Set a timeout for the entire socket
            send_humidity_data(s)
    except Exception as e:
        print(f"Error connecting to the gateway: {e}")
