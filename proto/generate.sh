cd proto
buf generate
cd ..

cp -r github.com/KYVENetwork/chain/* ./
rm -rf github.com

swagger-combine ./docs/config.json -o ./docs/swagger.yml
rm -rf tmp-swagger-gen
d