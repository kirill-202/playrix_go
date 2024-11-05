# Sync app for Google Sheets and Gridly

## Prerequisites
- Google Cloud project with Google Sheets API enabled.
- Create a Gridly project and obtain the API key from the app.
- Set up Google Sheets and Grids corresponding to one database in Gridly.
- Create a `.env` file in the project directory and add `client_secret.json` there.

## Usage
Ensure you have Docker and Make installed on your host.  
Run the following command to start the application:
```bash
make docker && make run
```

## How it works
The app updates at a set interval (default 20 seconds). It hashes each row in Google Sheets and the corresponding Grids in Gridly. If the hashes do not match, the entire row is updated. If Google Sheets contain more rows than Gridly, the missing rows will be added during the next update cycle.