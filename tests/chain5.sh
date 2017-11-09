init_path=$(pwd)
cd ../build
go build ../
go build ../client


./Gossiper -UIPort=5004 -gossipPort=localhost:10004 -name=nodeE > $init_path/E.log &
./Gossiper -UIPort=5003 -gossipPort=localhost:10003 -name=nodeD -peers=localhost:10004> $init_path/D.log &
./Gossiper -UIPort=5002 -gossipPort=localhost:10002 -name=nodeC -peers=localhost:10003> $init_path/C.log &
./Gossiper -UIPort=5001 -gossipPort=localhost:10001 -name=nodeB -peers=localhost:10002> $init_path/B.log &
./Gossiper -UIPort=5000 -gossipPort=localhost:10000 -name=nodeA -peers=localhost:10001 > $init_path/A.log &

sleep 1
./client -UIPort=5000 -msg="1:A->E"
sleep 2
./client -UIPort=5004 -msg="2:E->A"
sleep 10

# clearing
killall Gossiper
rm Gossiper client
cd $init_path
