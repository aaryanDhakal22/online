# ONLY FOR PRODUCTION
FROM nginx:alpine as dev
COPY ./nginx/prod.conf /etc/nginx/conf.d/default.conf
COPY ./frontend/ /usr/share/nginx/html/


