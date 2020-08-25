# Files Service

Files belonging to ants should be stored safely and be highly accessible around the world. Public files will be cached appropriately while private files will be encrypted at rest. All files belong to a profile and can be accessed by ant admins.

Generating the server:
----------------------
One requires :
    - docker installation to update the generated server from the api repo
    - Change directory one step above
    - Run the command to regenerate the client 
        `docker run --rm   -v ${PWD}:/local openapitools/openapi-generator-cli generate   -i /local/api/service/file/v1/file.yaml   -g go   -o /local/service-file-client`

    - To regenerate the server you need to run the command
        `docker run --rm   -v ${PWD}:/local openapitools/openapi-generator-cli generate   -i /local/api/service/file/v1/file.yaml   -g go-server --package-name openapi --additional-properties=sourceFolder=openapi  -o /local/service-file`
    