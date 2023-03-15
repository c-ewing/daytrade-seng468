# Test file for the DUMMY QUOTE SERVER

import socket

HOST = 'localhost'
PORT = 4444

def send_well_formed_quote():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.connect((HOST, PORT))
        s.sendall(b'BKM umhaEY4lil')

        data = s.recv(1024)
        print('[DEBUG] Received: ', data.decode("utf-8"))
        return data.decode("utf-8")
    

def test_well_formed_quote():
    response = send_well_formed_quote().split(",")
    assert float(response[0]) >= 0.5 and float(response[0]) <= 5.0
    assert response[1] == "BKM"
    assert response[2] == "umhaEY4lil"
    #assert response[3] skipped, because it is the timestamp
    assert response[4] == "Tnssjq2UKzc+KQ/KhjmENlfJSHRD7VBXxiYh1CVpyDo="