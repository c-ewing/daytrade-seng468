#!/usr/bin/env python
import pika
import time


def parse_file():
  file = open("user1.txt", "r")  
  for line in file :
    
    splitspace = line.split(" ")    
    print(splitspace[1].replace(","," ")) # ADD,oY01WVirLr,63511.53 becomes ADD oY01WVirLr 63511.53
    #send_and_print(splitspace[1].replace(","," "))
  


def send_and_print(command):
    channel.basic_publish(exchange='', routing_key='command_queue', body=command)
    print(" [x] Sent '" + command + "'")


connection = pika.BlockingConnection(
    pika.ConnectionParameters(host='localhost'))
channel = connection.channel()

channel.queue_declare(queue='command_queue', durable=True)

parse_file() #use this to call the send_and_print function for each line in the file

# send_and_print("ADD you 180.0")
# send_and_print("BUY you TSLA 1.0")
# send_and_print("COMMIT_BUY you")
connection.close()