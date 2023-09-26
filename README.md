# NeuralNexus API

## Economy DB

A cross platform currency/economy database for use across various platforms.

## Database Schema

```json
{
    "UserID": "0000-0000-0000-0000",
    "currencies": {
        "CurencyID": 0
    },
    "owned": [
        "CurrencyName"
    ]
}
```

Where `CurencyID` = `{OwnerID}_{CurrencyName}`

## Auth API

The main Auth API for NeuralNexus' APIs.
