# DePools

Auditing and auto switching infrastructure.

## Goals:
- Optimize latency for miners.
- Optimize reputation for miners.
- Minimize any extra setup for miners.

## Requirements:
- Verify the fees that DePool operators claim to charge.
- Quantify historical reputation including fees, uptime, etc.
- Measure latency between miners and DePools.

## Implementation:
### DePool operators:
- DePool operators will "register" with our backend with their claimed fee.
- Reputation key management:
  - This key will be generated within Pelagus with a new button called "Generate DePool ID"
  - The prefix will be 0xff to ensure that no money is stored on that address because the private key will need to live on the node.
- They will sign this registration message with their reputation key.
- When a miner connects to the DePool, the operator will sign any work that's broadcast to the miner.
- The signature will comprise of Sign(SealHash, Miner address)
- This signature will be included in the block's extra data, as well as sent to the miner separately.

### Miners:
- Upon startup, the miner will ping the DePool operators to measure latency.
- It will also consider the effective fee as reported by the backend.
- It will use a formula to combine the latency and the effective fee to determine overall profitability.
- The miner will choose the most "profitable" DePool as determined by the previous calculation.
- When a share is found, in addition to already sending the SealHash+nonce to the DePool node, the miner will also broadcast the signed Sign(SealHash, Miner address) to our backend, plus the Hash of the block they found.

### Backend:
- Our backend upon receipt will verify the following attributes of the submitted share:
  - the signature of SealHash+Address is valid is valid and attributable to a registered dePool;
  - the signed SealHash+Address are included in the share's extraData;
  - the provided Hash is included on chain;
- If all the information checks out, the backend will record a successfully broadcast share for that pool.
- The backend will also compare the coinbase in the actual workshare vs what was signed and included in the ExtraData and miner return.
- Using this information, the backend can calculate an effective fee and publish it in the statistics.
