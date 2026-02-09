# ONLY FOR PRODUCTION
FROM nginx:alpine as dev
COPY ./nginx/prod.conf /etc/nginx/conf.d/default.conf
COPY ./nginx/cloudflare-realip.conf /etc/nginx/conf.d/cloudflare-realip.conf
COPY ./frontend/ /usr/share/nginx/html/


