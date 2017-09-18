start:
	docker-compose build
	IP=$$(ifconfig | grep -E "([0-9]{1,3}\.){3}[0-9]{1,3}" | grep -v 127.0.0.1 | awk '{ print $$2 }' | cut -f2 -d: | head -n1) docker-compose up

stop:
	docker-compose down -v

