FROM cloudmatehub/wgdashboard:4.3.4
RUN /opt/wgdashboard/src/venv/bin/python /opt/cloudmatehub/test_broker.py
