LABEL org.opencontainers.image.source="https://github.com/OWNER/REPO"

FROM nginx:alpine

COPY . /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
