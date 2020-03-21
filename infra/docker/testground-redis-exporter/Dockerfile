FROM bitnami/redis-exporter

COPY grafana.ini /opt/bitnami/grafana/conf/grafana.ini
COPY dashboards/dashboards /opt/bitnami/grafana/conf/provisioning/dashboards
COPY dashboards/datasources /opt/bitnami/grafana/conf/provisioning/datasources
