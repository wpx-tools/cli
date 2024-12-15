FROM wordpress

RUN curl -O https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar
RUN chmod +x wp-cli.phar
RUN mv wp-cli.phar /usr/local/bin/wp

COPY bin/wpci /usr/local/bin/wpci
RUN nohup /usr/local/bin/wpci &