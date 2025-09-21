# Modern REST API development with Go

A project created by following the book:

https://learning.oreilly.com/library/view/modern-rest-api/9781836205371/


There are a few changes I have made to the project:

- Decided to do not implement echo framework because I wanted to focus on the fundamentals of the go language and the standard library.
- Instead of gorm I have used sqlc to generate the queries and the models.
- Used a different folder structure for the project to improve the organization.
- Used gomigrate to manage the database migrations.
- Used zerolog for logging.
- Used docker compose to manage the database for development.
- Used testcontainers to manage the database for testing.