# Private Key Authentication for Boop

This utility demonstrates how to authenticate with the Boop API using only a wallet private key, without requiring a browser or wallet extension.

## Building

Run the build script:

```
.\build.bat
```

This will create `auth_demo.exe` in the `bin` directory.

## Usage

```
auth_demo.exe <your-wallet-private-key>
```

Replace `<your-wallet-private-key>` with your Solana wallet private key in Base58 format.

## Integration in Your Application

There are three ways to use private key authentication in your application:

### 1. Direct Authentication

```go
// Get tokens directly from a private key
privyAuth, privyToken, privyRefresh, err := config.GetPrivyTokensWithPrivateKey(privateKey, logger)
if err != nil {
    // Handle error
}
```

### 2. Token Manager

```go
// Create a token manager that handles both authentication and token refresh
tokenManager, err := config.NewTokenManagerWithPrivateKey(privateKey, logger)
if err != nil {
    // Handle error
}

// Get tokens for API requests
graphqlToken := tokenManager.GetAuthorizationHeader()
```

### 3. Full Configuration

```go
// Create a complete configuration with all settings
cfg, err := config.NewConfigWithPrivateKey(privateKey)
if err != nil {
    // Handle error
}

// The config is now ready to use with all required tokens
```

## Security Warning

Never share or expose your private key. This authentication method should only be used in secure environments where you have full control over the private key storage. 