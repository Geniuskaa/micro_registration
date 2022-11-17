# Registration service setup

This service is the part of microservice ecosystem. 
The purpose of the whole ecosystem is system to conduct karate competition 
and visualization achievements of each member of competition.

## Getting started

To properly start app you need to add `.env` file to `./configs` directory or
you can just write all neccescary environment variables start with
`ALT_` prefix. For example: to write `DB_PORT` variable you need
to name it `ALT_DB_PORT`

Also you should know that if you would use both: configuration file and environment variables.
Environment variables take more precedence and app will use it instead of config file values