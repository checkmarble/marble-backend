# An invalid UUID returns a 400

GET http://localhost/v1beta/decisions/nouuid/sanction-checks
x-api-key: testapikey

HTTP 400

[Asserts]
jsonpath "$.error.messages" nth 0 contains "UUID"

# A non-existent decision returns 404

GET http://localhost/v1beta/decisions/00000000-0000-0000-0000-000000000000/sanction-checks
x-api-key: testapikey

HTTP 404

[Asserts]
jsonpath "$.error.messages" nth 0 contains "does not exist"

# Get a sanction check from decision

GET http://localhost/v1beta/decisions/11111111-1111-1111-1111-111111111111/sanction-checks
x-api-key: testapikey

HTTP 200

[Asserts]
jsonpath "$.data" count == 1
jsonpath "$.data[0].id" == "11111111-1111-1111-1111-111111111111"
jsonpath "$.data[0].status" == "in_review"
jsonpath "$.data[0].query.lorem" == "ipsum"
jsonpath "$.data[0].matches" count == 3
jsonpath "$.data[0].matches[0].id" == "11111111-1111-1111-1111-111111111111"
jsonpath "$.data[0].matches[0].status" == "pending"
jsonpath "$.data[0].matches[1].id" == "22222222-2222-2222-2222-222222222222"
jsonpath "$.data[0].matches[1].status" == "no_hit"

# A decision without a sanction check returns a 404

GET http://localhost/v1beta/decisions/22222222-2222-2222-2222-222222222222/sanction-checks
x-api-key: testapikey

HTTP 404

[Asserts]
jsonpath "$.error.messages" nth 0 contains "does not have a sanction check"

# We can change the match status of a pending match

POST http://localhost/v1beta/sanction-checks/matches/11111111-1111-1111-1111-111111111111
x-api-key: testapikey

{
    "status": "no_hit"
}

HTTP 200

[Asserts]
jsonpath "$.data.id" == "11111111-1111-1111-1111-111111111111"
jsonpath "$.data.status" == "no_hit"

# But we cannot of an already reviewed match

POST http://localhost/v1beta/sanction-checks/matches/22222222-2222-2222-2222-222222222222
x-api-key: testapikey

{
    "status": "no_hit"
}

HTTP 422

[Asserts]
jsonpath "$.error.messages" nth 0 contains "not pending review"
