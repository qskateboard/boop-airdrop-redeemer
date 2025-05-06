#!/bin/bash
# Auto Claimer for Boop airdrops on Linux/macOS

# Set your wallet information
# Set your private key for automatic authentication
export WALLET_PRIVATE_KEY="YOUR_WALLET_PRIVATE_KEY"

# Wallet address is optional when using private key, it will be derived automatically
export WALLET_ADDRESS="YOUR_WALLET_ADDRESS"

# Set your Solana RPC URL (default is the public endpoint but it has rate limits)
export SOLANA_RPC_URL="https://api.mainnet-beta.solana.com"

# Enable debug mode for verbose logging (optional)
export DEBUG="true"

# Set environment variables for configuration
export MINIMUM_USD_THRESHOLD=0.15
export BOOP_API_URL=https://graphql-mainnet.boop.works/graphql

# Telegram notifications configuration
export ENABLE_TELEGRAM=true
export TELEGRAM_BOT_TOKEN=your_bot_token_here
export TELEGRAM_CHAT_ID=your_chat_id_here

echo "Starting Boop Auto Claimer..."
echo "Solana RPC: $SOLANA_RPC_URL"
echo "Minimum USD Threshold: $MINIMUM_USD_THRESHOLD"

# Run the auto claimer
./airdrop-redeemer 