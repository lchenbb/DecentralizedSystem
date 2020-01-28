pkill -f Peerster

# Build executable
go build -o PeersterExe ./Peerster/main.go
go build -o client ./Peerster/client/main.go
rm *.txt

# Start gossiper
./PeersterExe -name A -N 3 -gossipAddr 127.0.0.1:5000 -UIPort 8080 -peers 127.0.0.1:5001,127.0.0.1:5002 > A.txt &
./PeersterExe -name B -N 3 -gossipAddr 127.0.0.1:5001 -UIPort 8081 -peers 127.0.0.1:5000,127.0.0.1:5002  > B.txt &
./PeersterExe -name C -N 3 -gossipAddr 127.0.0.1:5002 -UIPort 8082 -peers 127.0.0.1:5000,127.0.0.1:5001 > C.txt &
sleep 3

# Start client
# Round 1
./client -UIPort 8080 -Voterid 1 -Vote Trump -Election States &
./client -UIPort 8081 -Voterid 1 -Vote Trump -Election States &
./client -UIPort 8082 -Voterid 2 -Vote Clinton -Election States &
./client -UIPort 8080 -Voterid 3 -Vote Bush -Election States &
./client -UIPort 8082 -Voterid 6 -Vote Mao -Election China &
./client -UIPort 8081 -Voterid 8 -Vote Deng -Election China &
./client -UIPort 8080 -Voterid 1 -Vote Moon	-Election Korea &
./client -UIPort 8081 -Voterid 2 -Vote Kim -Election Korea &
sleep 3

# Round 2
./client -UIPort 8080 -Voterid 2 -Vote Clinton -Election States &
./client -UIPort 8082 -Voterid 3 -Vote Bush -Election States &
./client -UIPort 8080 -Voterid 3 -Vote Lee -Election Korea &
sleep 3

# Round 3
./client -UIPort 8080 -Voterid 4 -Vote Washington -Election States &
./client -UIPort 8081 -Voterid 5 -Vote Reagon -Election States &
./client -UIPort 8082 -Voterid 7 -Vote Xi -Election China &
sleep 60

pkill -f Peerster
