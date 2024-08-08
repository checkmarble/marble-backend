Syncronize the openapi spec to the webpage's API documentation with readme:

```
npm install rdme@latest -g
rdme openapi
```

This will require you to :

1. login with readme if you have a user/password.
2. enter the subdomain used (= checkmarble)
3. select the API definition you want to push
4. select "Update an existing OAS file"
5. select the desired file to update (**be careful to select the right one**)

> **⚠️ Ensure you sync the selected OAS with the corresponding API Reference or you will loose any existing manual edition in the process ⚠️**

For more information on the readme openapi extension, see:
<https://docs.readme.com/main/docs/openapi-extensions>
