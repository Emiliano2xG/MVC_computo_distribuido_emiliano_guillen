FROM nginx:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY index.html estilos.css app.js /usr/share/nginx/html/
COPY brand /usr/share/nginx/html/brand
EXPOSE 80
