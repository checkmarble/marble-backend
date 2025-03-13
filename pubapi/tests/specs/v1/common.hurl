# Sanity check to check if API is up

GET http://localhost/liveness
HTTP 200

# Invalid API key should return an error

POST http://localhost/v1beta/example
x-api-key: wrongapikey

HTTP 401

# Dummy test on dummy endpoint

POST http://localhost/v1beta/example
x-api-key: testapikey

{
    "age": 22,
    "email": "test@example.com",
    "is_nice": "true"
}

HTTP 200
