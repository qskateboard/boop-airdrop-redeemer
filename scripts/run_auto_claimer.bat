@echo off

REM Set environment variables for configuration
set MINIMUM_USD_THRESHOLD=0.15
set BOOP_API_URL=https://graphql-mainnet.boop.works/graphql

REM Telegram notifications configuration
set ENABLE_TELEGRAM=true
set TELEGRAM_BOT_TOKEN=YOUR_TELEGRAM_BOT_TOKEN
set TELEGRAM_CHAT_ID=YOUR_TELEGRAM_CHAT_ID

REM Set your wallet private key for automatic authentication (RECOMMENDED)
set WALLET_PRIVATE_KEY=YOUR_WALLET_PRIVATE_KEY

REM Wallet address is optional when using private key, it will be derived automatically
set WALLET_ADDRESS=YOUR_WALLET_ADDRESS

REM Set your Solana RPC URL (default is the public endpoint but it has rate limits)
set SOLANA_RPC_URL=https://api.mainnet-beta.solana.com

REM Set the interval for checking for new airdrops (1 minute)
set CHECK_INTERVAL=1m

set STATS_DATA_DIR=./data/stats

REM Enable debug mode for verbose logging (optional)
set DEBUG=true


echo Starting Boop Auto Claimer...
echo Minimum USD Threshold: %MINIMUM_USD_THRESHOLD%
echo Solana RPC: %SOLANA_RPC_URL%

REM Build and run the auto-claimer
cd %~dp0..
go run cmd/auto_claim/main.go

pause 