# Use a Gremlin server version compatible with gremlin-go
FROM tinkerpop/gremlin-server:3.7.4

# Set working directory
WORKDIR /opt/gremlin-server

# Copy custom configuration files
COPY gremlin-config/ /opt/gremlin-server/conf/
# Expose Gremlin server port
EXPOSE 8182
