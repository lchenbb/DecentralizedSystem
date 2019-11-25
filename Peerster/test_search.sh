pkill -f Peerster
go build
cd _Downloads
rm *
cd ..
cd client
go build
cd ..

# Build a square connected topology
./Peerster -name A -gossipAddr 127.0.0.1:5001 -rtimer 5 -UIPort 8080 -peers 127.0.0.1:5002,127.0.0.1:5004 > A.txt &

./Peerster -name B -gossipAddr 127.0.0.1:5002 -rtimer 5 -UIPort 8081 -peers 127.0.0.1:5001,127.0.0.1:5003 > B.txt &

./Peerster -name C -gossipAddr 127.0.0.1:5003 -rtimer 5 -UIPort 8082 -peers 127.0.0.1:5002,127.0.0.1:5004 > C.txt &

./Peerster -name D -gossipAddr 127.0.0.1:5004 -rtimer 5 -UIPort 8083 -peers 127.0.0.1:5001,127.0.0.1:5003 > D.txt &

# Instruct peer to construct index file
cd client
./client -UIPort 8080 -file A_FILE
./client -UIPort 8081 -file B_FILE
./client -UIPort 8082 -file C_FILE
./client -UIPort 8083 -file D_FILE

# Allow peers to exchange routing information
sleep 10

# Trigger file search at A
./client -UIPort 8080 -keywords FILE &

cd ..
sleep 20
pkill -f Peerster
cd client 
rm client 
cd ..

