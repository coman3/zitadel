#!/bin/bash

# ------------------------------
# prints a message as soon as 
# ZITADEL is ready
# ------------------------------

be_status=""
env_status=""
console_status=""

while [[ $be_status != 200 && $env_status != 200 ]]; do
    sleep 5
    ## This is a workaround for a race condition
    if [[ $be_status -eq 412 ]]; then
        echo "please restart the process once again to get rid of the 412 error!"
    fi
    be_status=$(curl -s -o /dev/null -I -w "%{http_code}" host.docker.internal:${BE_PORT}/clientID)
    env_status=$(curl -s -o /dev/null -I -w "%{http_code}" host.docker.internal:${FE_PORT}/assets/environment.json)
    echo "backend (${be_status}) or environment (${env_status}) not ready yet ==> retrying in 5 seconds"
done

echo "backend and environment.json ready!"

while [[ $console_status != 200 ]]; do
    sleep 15
    console_status=$(curl -s -o /dev/null -I -w "%{http_code}" host.docker.internal:${FE_PORT}/index.html)
    echo "console (${console_status}) not ready yet ==> retrying in 15 seconds"
done

echo "console ready - please wait shortly!"
sleep 15

echo -e "++=======================================================================================++
||                                                                                       ||
|| ZZZZZZZZZZZZ II TTTTTTTTTTTT       AAAA       DDDDDD     EEEEEEEEEE LL                ||
||          ZZ  II      TT           AA  AA      DD    DD   EE         LL                ||
||        ZZ    II      TT          AA    AA     DD      DD EE         LL                ||
||      ZZ      II      TT         AA      AA    DD      DD EEEEEEEE   LL                ||
||    ZZ        II      TT        AAAAAAAAAAAA   DD      DD EE         LL                ||
||  ZZ          II      TT       AA          AA  DD    DD   EE         LL                ||
|| ZZZZZZZZZZZZ II      TT      AA            AA DDDDDD     EEEEEEEEEE LLLLLLLLLL        ||
||                                                                                       ||
||                                                                                       ||
|| SSSSSSSSSS TTTTTTTTTTTT       AAAA       RRRRRRRR TTTTTTTTTTTT  EEEEEEEEEE DDDDDD     ||
|| SS              TT           AA  AA      RR    RR      TT       EE         DD    DD   ||
||  SS             TT          AA    AA     RR    RR      TT       EE         DD      DD ||
||   SSSSSS        TT         AA      AA    RRRRRRRR      TT       EEEEEEEE   DD      DD ||
||        SS       TT        AAAAAAAAAAAA   RRRR          TT       EE         DD      DD ||
||         SS      TT       AA          AA  RR  RR        TT       EE         DD    DD   ||
|| SSSSSSSSSS      TT      AA            AA RR    RR      TT       EEEEEEEEEE DDDDDD     ||
||                                                                                       ||
++=======================================================================================++"

echo "access the console here http://localhost:${FE_PORT}"
echo "access the login here http://localhost:50003/login"
echo "access the apis here http://localhost:50002"
