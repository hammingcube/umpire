container=$(docker run -d -p 3000:1323 dkr-umpire)
sleep 4
output=$(curl -H "Content-Type: application/json" -X POST http://localhost:3000/execute -d @body.json)
expected='{"status":"pass","details":"","stdout":"hello\nhello","stderr":""}'
if [ "$output" = "$expected" ]
then
	echo "got expected output"
	echo $output
else
	echo "got unexpected output"
	echo $output
	echo "was expecting"
	echo $expected
fi

docker stop $container
docker rm $container

