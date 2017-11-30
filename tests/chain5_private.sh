init_path=$(pwd)
cd ../build
go build ../
go build ../client

./Gossiper -UIPort=5004 -gossipAddr=localhost:10004 -name=nodeE > $init_path/E.log &
sleep 0.5
./Gossiper -UIPort=5003 -gossipAddr=localhost:10003 -name=nodeD -peers=localhost:10004> $init_path/D.log &
sleep 0.5
./Gossiper -UIPort=5002 -gossipAddr=localhost:10002 -name=nodeC -peers=localhost:10003> $init_path/C.log &
sleep 0.5
./Gossiper -UIPort=5001 -gossipAddr=localhost:10001 -name=nodeB -peers=localhost:10002> $init_path/B.log &
sleep 0.5
./Gossiper -UIPort=5000 -gossipAddr=localhost:10000 -name=nodeA -peers=localhost:10001 > $init_path/A.log &

GREEN='\033[0;32m'
msg1='A->E'
msg2='E->A'

sleep 5
./client -UIPort=5000 -msg="$msg1" -Dest=nodeE
sleep 2
./client -UIPort=5004 -msg="$msg2" -Dest=nodeA
sleep 2

# clearing
killall Gossiper
rm Gossiper client
cd $init_path

# Analysing
if grep -q "$msg1" E.log ; then
	echo -e "${GREEN}$msg1 succed"
fi
if grep -q "$msg2" A.log ; then
	echo -e "${GREEN}$msg2 succed"
fi
if grep -q "ID 0" *.log ; then
	echo "ID=0 found"
fi
