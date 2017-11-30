#!/usr/bin/env bash
set -e

cd ../build
source build.sh
mv Gossiper gossiper

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'
DEBUG="false"

outputFiles=()
file=testFile.txt
path_logs=../tests

# Making a simple network:
#   A - B

startGossip(){
	local name=$1
	local port=$2
	local peers=""
	local workingPath="$path_logs/$name/"

	if [ ! -d "$workingPath" ]; then
		mkdir -p "$workingPath/_Downloads"
	fi

	if [ "$3" ]; then
		peers="-peers=127.0.0.1:$3"
	fi
	echo ./gossiper -gossipAddr=127.0.0.1:$port -UIPort=$((port+1)) -name=$name $peers
	./gossiper -gossipAddr=127.0.0.1:$port -UIPort=$((port+1)) -name=$name $peers -workingPath=$workingPath> $path_logs/$name.log &
	# don't show 'killed by signal'-messages
	disown
}
startGossip A 10000
startGossip B 10002 10000
sleep 1

# Share file with A
cp "$path_logs/testFile.txt" "$path_logs/A"
./client -UIPort=10001 -file=$file
sleep 1

# Get file from B
./client -UIPort=10003 -file=$file -Dest=A -request=ec3326e48292135e90b516771822038fc55422ccfe2ebce6505f39a688ef5f7e 
sleep 4

#testing
fail(){
	echo -e "${RED}*** Failed test $1 ***${NC}"
  	killall gossiper
	exit 1
}

grep -q "RECONSTRUCTED file" $path_logs/B.log || fail "File not reconstructed"
diff -q $path_logs/A/$file $path_logs/B/_Downloads/$file || fail "Files are not similar"

echo -e "${GREEN}***SUCCESS***${NC}"

killall gossiper
rm gossiper client
