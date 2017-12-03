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
file=testFile
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
cp "$path_logs/$file" "$path_logs/A"
./client -UIPort=10001 -file=$file
sleep 1

# Get file from B
./client -UIPort=10003 -keywords "te,fi" -budget 2 
sleep 9 
killall gossiper

#testing
fail(){
	echo -e "${RED}*** Failed test $1 ***${NC}"
	exit 1
}

grep -q "SEARCH FINISHED" $path_logs/B.log || fail "File not found"

echo -e "${GREEN}***SUCCESS***${NC}"

rm gossiper client
