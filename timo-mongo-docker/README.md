# Mongo Setup for TIMO

```
cd timo-mongo-docker
cd sharding
docker network create timo-docker-network
docker pull mongo

docker-compose -f config-server/docker-compose.yaml up -d
docker-compose -f shard1/docker-compose.yaml up -d
docker-compose -f shard2/docker-compose.yaml up -d

docker run -d 35000:27107 --name="timoserver" --network="timo-docker-network" -v data:/data/db mongo   

docker exec -it timoserver bash 
```

then inside that terminal run 

```mongos --configdb cfgrs/172.22.0.2:27017,172.22.0.3:27017,172.22.0.4:27017 --port 27011```

then open a new terminal as that one will be running mongos

then in that new terminal `cd` as shown before, and from there 

```docker exec -it timoserver bash ```

and in that shell

```mongosh localhost:27011```

then to check you can do `show dbs`

and if timo shows up then it worked!

![image](https://user-images.githubusercontent.com/85471657/223895197-dd29a5f8-a1cc-4796-818d-99b0f775a294.png)


