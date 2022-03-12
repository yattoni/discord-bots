GOOS=linux GOARCH=amd64 go build -o lambda-gas gas/gas-bot.go;
zip lambda-gas.zip lambda-gas
mkdir -p bin
mv lambda-gas lambda-gas.zip bin
