#!/usr/bin/env fish

function login
    set result (curl \
        -s \
        -X POST \
        -H "Content-Type: application/json" \
        --data '{"username": "kpors", "password": "test"}' \
        'http://localhost:8080/api/v1/login')

    echo $result | jq
    set -Ux VITA_JWT (echo $result | jq -r .jwt)
end

function login_wrong_credentials
    set result (curl \
        -s \
        -X POST \
        -H "Content-Type: application/json" \
        --data '{"username": "foo", "password": "quux"}' \
        'http://localhost:8080/api/v1/login')

    echo $result | jq
end

function post_new_cv
    set result (curl \
        -s \
		-vv \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $VITA_JWT" \
        --data @./entry.json \
        'http://localhost:8080/api/v1/cv')

    echo $result
end

function post_new_incorrect_cv
    set result (curl \
        -s \
        -X POST \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $VITA_JWT" \
        --data '{}' \
        'http://localhost:8080/api/v1/cv')

    echo $result | jq
end


function put_dev_version
    set result (curl \
        -s \
        -X PUT \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $VITA_JWT" \
        --data @./entry.json \
        'http://localhost:8080/api/v1/cv/development')

    echo $result | jq
end


function get_current_cv_json
	echo $VITA_JWT

    set result (curl \
		-vv \
        -s \
        -H "Accept: application/json" \
        -H "Authorization: Bearer $VITA_JWT" \
        'http://localhost:8080/api/v1/cv')

    echo $result | jq
end


function get_current_cv_pdf
    curl \
        -v \
        -H "Accept: application/pdf" \
        -H "Authorization: Bearer $VITA_JWT" \
        'http://localhost:8080/api/v1/cv' \
        --output current_cv.pdf
end

function get_revisions
    set result (curl \
        -v \
        -H "Accept: application/json" \
        -H "Authorization: Bearer $VITA_JWT" \
        'http://localhost:8080/api/v1/cv/revisions')

    echo $result
end

function post_preview
    set preview (jq -c '{ "template": "q.typ" , "cv": . }' ./entry.json)

    echo $preview

    curl \
        -v \
        -H "Accept: application/pdf" \
        -H "Authorization: Bearer $VITA_JWT" \
        --data {$preview} \
        'http://localhost:8080/api/v1/cv/preview' \
        --output preview.pdf
end

function delete_revisions
    set result (curl \
        -v \
        -X DELETE \
        -H "Accept: application/json" \
        -H "Authorization: Bearer $VITA_JWT" \
        'http://localhost:8080/api/v1/cv/revisions')

    echo $result
end

function get_development_cv_json
    set result (curl \
        -v \
        -H "Accept: application/json" \
        -H "Authorization: Bearer $VITA_JWT" \
        'http://localhost:8080/api/v1/cv/development')

    echo $result | jq
end

function get_development_cv_pdf
    curl \
        -v \
        -H "Accept: application/pdf" \
        -H "Authorization: Bearer $VITA_JWT" \
        'http://localhost:8080/api/v1/cv/development' \
        --output current_cv_development.pdf
end

$argv[1]
