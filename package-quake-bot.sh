GOOS=linux GOARCH=amd64 go build -o lambda-quake quake/quake-bot.go;
zip lambda-quake.zip lambda-quake
mkdir -p bin
mv lambda-quake lambda-quake.zip bin
