FROM ghcr.io/wgdashboard/wgdashboard:v4.3.4@sha256:8c5b550db4fda04eff06e96af801499be485c73e0912fcf0c1031babca0f7e7b
COPY integrations/wgdashboard /opt/cloudmatehub
RUN cp /opt/cloudmatehub/seed_key.py /opt/wgdashboard/src/cloudmatehub_seed_key.py && \
    printf '\nimport cloudmatehub_seed_key\n' >> /opt/wgdashboard/src/gunicorn.conf.py && \
    /opt/wgdashboard/src/venv/bin/python -c "from pathlib import Path; p=Path('/opt/wgdashboard/src/dashboard.py'); s=p.read_text(); old=' - Key: {apiKey}'; assert old in s; p.write_text(s.replace(old, ''))"
ENTRYPOINT ["/bin/sh", "/opt/cloudmatehub/entrypoint.sh"]
