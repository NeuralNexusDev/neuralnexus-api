# NeuralNexus API

## economy-db

A cross platform currency/economy database for use across various platforms.

## Database Schema

```json
"UserID": {
    "currencies": {
        "CurencyID": 0
    },
    "owned": [
        "CurrencyName"
    ]
}
```

Where `CurencyID` = `{OwnerID}_{CurrencyName}`
