oha -z 30s -c 500 http://localhost:8080/accounts/alice
oha -z 30s -c 200 --no-tui http://localhost:8080/balance/alice

oha -z 30s -c 200 -m POST -d '{"from":"alice","to":"bob","amount":1}'     http://localhost:8080/transfers
