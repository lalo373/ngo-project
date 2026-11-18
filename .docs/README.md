# TECH STACK
backend -> go
architecture -> monolith
frontend -> typescript-react: normal white and black modern themed
db -> postgreSQL(neon)

# roles
1. executives
2. supervisors 
    only paid daily if 70% executive are eligible for daily Payout
    maximum 20 executives per supervisor
3. district coordinators 
    only paid daily if 85% supervisors are eligible for daily Payout
    maximum 6 supervisors per district coordinator
4. regional coordinators
    maximum 6 district coordinator per regional coordinator


## RULES
1. executive: if executive logined btw 9am-5pm eligible to daily payout
2. supervisors if 70% executive

# functionality

## @executive: only can view theie payouts
1. api: POST /login/executive should login by id and password
2. api: GET /dailyPayout 
    only eligible for daily pay if logined within 9am to 5pm and completed 25 target then only are eligible for daily payout, added amount depends like this 
    if target_count >= 25 and target_count <  45 {
    weeklyPayout += 500
    }
    if target_count >= 45 {
    weeklyPayout += 1000
    }
    amount showed in (money in rupee symbol)
3. api: GET /weeklyPayouts should display every week (monday-sunday dates) in descending order , display current week on top, 
4. api: GET /calendar
    drop down for months,
    display tabular data (date, payout),

> OUR backend should compute every eligibility by 8:00 PM daily (thats why using go for concurrency, possible calculations can be like 10000 every day)

## @supervisor: only can view theie payouts
1. api: POST /login/supervisor should login by id and password (password given by admin)
2. api: GET /dailyPayout
    only paid daily if 70% executive are eligible for daily Payout
    are eligible for daily payout, showed in  (money in rupee symbol)
3. api: GET /weeklyPayouts 
    should display every week (monday-sunday dates) in descending order , 
    display current week on top,
4. api: GET /calendar
    drop down for months
    display tabular data (date, payout)
    
## @district_coordinator: only can view theie payouts
1. api: POST /login/district_coordinator should login by id and password
2. api: GET /dailyPayout 
    only paid daily if 85% supervisors are eligible for daily Payout
    are eligible for daily payout, showed in  (money in rupee symbol)
3. api: GET /weeklyPayouts 
    should display every week (monday-sunday dates) in descending order , 
    display current week on top, 
4. api: GET /calendar
    drop down for months
    display tabular data (date, payout)

## @admin: add, delete edit all members
0. POST: /admin/login
1. GET: /admin/dashboard/getDataByDate download daily saved daily reports from database into csv files like logs, should be able to get by date

temproary problem: now we dont have csv, data or api to get any updates of executive data like daily target_count, so we need some dummy csv data to start with (a week's data of 100 executives, 5 supervisors, 1 district coordinator)

there should be database with hashed passwords to check every login
all sessions should be logged out after 24 hours
