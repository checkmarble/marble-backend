POST http://localhost/v1beta/sanction-checks/whitelists
x-api-key: testapikey

{
    "counterparty": "Jean-Baptiste Zorg",
    "entity_id": "ABC123"
}

HTTP 201

POST http://localhost/v1beta/sanction-checks/whitelists
x-api-key: testapikey

{
    "counterparty": "Joe Bill",
    "entity_id": "ABC123"
}

POST http://localhost/v1beta/sanction-checks/whitelists
x-api-key: testapikey

{
    "counterparty": "JBZ",
    "entity_id": "ABC123"
}

HTTP 201

POST http://localhost/v1beta/sanction-checks/whitelists/search
x-api-key: testapikey

{
    "entity_id": "ABC123"
}

HTTP 200

[Asserts]
jsonpath "$.data" count == 3
jsonpath "$.data[*].entity_id" contains "ABC123"
jsonpath "$.data[*].counterparty" contains "Jean-Baptiste Zorg"
jsonpath "$.data[*].counterparty" contains "JBZ"
jsonpath "$.data[*].counterparty" contains "Joe Bill"

DELETE http://localhost/v1beta/sanction-checks/whitelists
x-api-key: testapikey

{
    "counterparty": "Joe Bill",
    "entity_id": "ABC123"
}

HTTP 204

POST http://localhost/v1beta/sanction-checks/whitelists/search
x-api-key: testapikey

{
    "entity_id": "ABC123"
}

HTTP 200

[Asserts]
jsonpath "$.data" count == 2
jsonpath "$.data[*].counterparty" not contains "Joe Bill"

DELETE http://localhost/v1beta/sanction-checks/whitelists
x-api-key: testapikey

{
    "entity_id": "ABC123"
}

POST http://localhost/v1beta/sanction-checks/whitelists/search
x-api-key: testapikey

{
    "entity_id": "ABC123"
}

HTTP 200

[Asserts]
jsonpath "$.data" count == 0