# NeuralNexus API

## Economy API

A cross platform currency/economy database for use across various platforms.

### Economy Schema

```json
{
    "userId": "0000-0000-0000-0000",
    "currencies": {
        "CurencyID": 0
    },
    "owned": [
        "CurrencyName"
    ]
}
```

Where `CurencyID` = `{OwnerID}_{CurrencyName}`

## Authentication API

The main Auth API for NeuralNexus' APIs.

### Authentication Schema

```json
{
    "userId": "0000-0000-0000-0000",
    "hashedSecret": "00a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0",
    "tokens": [
        "wadawd.wdwadwadawdawd.wadawdawdawd"
    ]
}
```

## Accounts API

General user account management and linkage.

### Accounts Schema

```json
{
    "userId": "0000-0000-0000-0000",
    "discord": {
        "id": "000000000000000000",
        "username": "Username#0000",
        "avatar": "000",
        "...": "..."
    },
    "twitch" {
        "id": "0000000000",
        "login": "Username",
        "display_name": "Display Name",
        "...": "..."
    },
    "minecraft": {
        "id": "3cf69d1b-a45a-4c19-9ff7-ac1e3bafec6b",
        "username": "metalcatian",
        "skin": "https://crafatar.com/skins/3cf69d1b-a45a-4c19-9ff7-ac1e3bafec6b"
    }
}
```
