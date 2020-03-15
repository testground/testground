module.exports = {
  "versions": [{
    "name": "old-bitswap",
    "ref": "dcfe40e",
    "testgroundId": "haves_support_no"
  }, {
    "name": "master",
    "ref": "master",
    "testgroundId": "haves_support_yes"
  }],
  "suites": {
    "old-from-2old": {
      "seed": [{
        "count": 2,
        "version": "old-bitswap"
      }],
      "leech": [{
        "count": 1,
        "version": "old-bitswap"
      }]
    },
    "master-from-2master": {
      "seed": [{
        "count": 2,
        "version": "master"
      }],
      "leech": [{
        "count": 1,
        "version": "master"
      }]
    },
    "master-from-2old": {
      "seed": [{
        "count": 2,
        "version": "old-bitswap"
      }],
      "leech": [{
        "count": 1,
        "version": "master"
      }]
    },
    "master-from-1master,1old": {
      "seed": [{
        "count": 1,
        "version": "old-bitswap"
      }, {
        "count": 1,
        "version": "master"
      }],
      "leech": [{
        "count": 1,
        "version": "master"
      }]
    },
  }
}
