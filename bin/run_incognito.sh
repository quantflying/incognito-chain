#!/bin/sh
cron



if [ -z "$NAME" ]; then
    NAME="miner"
fi

if [ "$1" = "y" ]; then
  echo "clear...."
    find /data -maxdepth 2 -mindepth 1 -type d | xargs rm -rf
fi

mkdir -p /data

if [ -z "$TESTNET" ]; then
    TESTNET=true;
fi

if [ -z $BOOTNODE_IP ]; then
    BOOTNODE_IP="testnet-bootnode.incognito.org:9330";
fi

if [ -z "$NODE_PORT" ]; then
    NODE_PORT=9433;
fi

if [ -z "$LIMIT_FEE" ]; then
    LIMIT_FEE=1;
fi

if [ -z "$PUBLIC_IP" ]; then
    PUBLIC_IP=`dig -4 @resolver1.opendns.com ANY myip.opendns.com +short`;
fi

if [ -z "$RPC_PORT" ]; then RPC_PORT=9334; fi

if [ -z "$WS_PORT" ]; then WS_PORT=19334; fi

if [ -n "$FULLNODE" ] &&  [ "$FULLNODE" = "1" ]; then
    echo ./incognito --nodemode "relay" --relayshards "all" -n $NAME --discoverpeers --discoverpeersaddress $BOOTNODE_IP --nodemode "relay" --datadir "/data" --listen "0.0.0.0:$NODE_PORT" --externaladdress "$PUBLIC_IP:$NODE_PORT" --enablewallet --wallet "wallet" --walletpassphrase "12345678" --walletautoinit --testnet $TESTNET --limitfee $LIMIT_FEE --norpcauth --rpclisten "0.0.0.0:$RPC_PORT" --rpcwslisten "0.0.0.0:$WS_PORT" --loglevel "info" > cmd.sh
   ./incognito --nodemode "relay" --relayshards "all" -n $NAME --discoverpeers --discoverpeersaddress $BOOTNODE_IP --nodemode "relay" --datadir "/data" --listen "0.0.0.0:$NODE_PORT" --externaladdress "$PUBLIC_IP:$NODE_PORT" --enablewallet --wallet "wallet" --walletpassphrase "12345678" --walletautoinit --testnet $TESTNET --limitfee $LIMIT_FEE --norpcauth --rpclisten "0.0.0.0:$RPC_PORT" --rpcwslisten "0.0.0.0:$WS_PORT" --loglevel "info" --rpcmaxclients 1500 2>/data/error_log.txt | cronolog /data/$PUBLIC_IP-%Y-%m-%d.log -S /data/$PUBLIC_IP.cur.log
elif [ -n "$PRIVATEKEY" ]; then
    echo ./incognito -n $NAME --testnet $TESTNET --limitfee $LIMIT_FEE --discoverpeers --discoverpeersaddress $BOOTNODE_IP --privatekey $PRIVATEKEY --nodemode "auto" --datadir "/data" --listen "0.0.0.0:$NODE_PORT" --externaladdress "$PUBLIC_IP:$NODE_PORT" --norpcauth --enablewallet --wallet "incognito" --walletpassphrase "12345678" --walletautoinit --rpclisten "0.0.0.0:$RPC_PORT" --rpcwslisten "0.0.0.0:$WS_PORT" --metricurl "$METRIC_URL" --loglevel "info" --btcclient 1 --btcclientip "159.65.142.153" --btcclientport "8332" --btcclientusername "admin" --btcclientpassword "autonomous" > cmd.sh
    ./incognito -n $NAME --testnet $TESTNET --limitfee $LIMIT_FEE --discoverpeers --discoverpeersaddress $BOOTNODE_IP --privatekey $PRIVATEKEY --nodemode "auto" --datadir "/data" --listen "0.0.0.0:$NODE_PORT" --externaladdress "$PUBLIC_IP:$NODE_PORT" --norpcauth --enablewallet --wallet "incognito" --walletpassphrase "12345678" --walletautoinit --rpclisten "0.0.0.0:$RPC_PORT" --rpcwslisten "0.0.0.0:$WS_PORT" --metricurl "$METRIC_URL"  --loglevel "info" --btcclient 1 --btcclientip "159.65.142.153" --btcclientport "8332" --btcclientusername "admin" --btcclientpassword "autonomous" 2>/data/error_log.txt | cronolog /data/$PUBLIC_IP-%Y-%m-%d.log -S /data/$PUBLIC_IP.cur.log
elif [ -n "$MININGKEY" ]; then
    echo ./incognito -n $NAME --testnet $TESTNET --limitfee $LIMIT_FEE --discoverpeers --discoverpeersaddress $BOOTNODE_IP --miningkeys $MININGKEY --nodemode "auto" --datadir "/data" --listen "0.0.0.0:$NODE_PORT" --externaladdress "$PUBLIC_IP:$NODE_PORT" --norpcauth --enablewallet --wallet "incognito" --walletpassphrase "12345678" --walletautoinit --rpclisten "0.0.0.0:$RPC_PORT" --rpcwslisten "0.0.0.0:$WS_PORT" --metricurl "$METRIC_URL" --loglevel "info" --btcclient 1 --btcclientip "159.65.142.153" --btcclientport "8332" --btcclientusername "admin" --btcclientpassword "autonomous" > cmd.sh
    ./incognito -n $NAME --testnet $TESTNET --limitfee $LIMIT_FEE --discoverpeers --discoverpeersaddress $BOOTNODE_IP --miningkeys $MININGKEY --nodemode "auto" --datadir "/data" --listen "0.0.0.0:$NODE_PORT" --externaladdress "$PUBLIC_IP:$NODE_PORT" --norpcauth --enablewallet --wallet "incognito" --walletpassphrase "12345678" --walletautoinit --rpclisten "0.0.0.0:$RPC_PORT" --rpcwslisten "0.0.0.0:$WS_PORT" --metricurl "$METRIC_URL" --loglevel "info" --btcclient 1 --btcclientip "159.65.142.153" --btcclientport "8332" --btcclientusername "admin" --btcclientpassword "autonomous" 2>/data/error_log.txt | cronolog /data/$PUBLIC_IP-%Y-%m-%d.log -S /data/$PUBLIC_IP.cur.log
fi
