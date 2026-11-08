# Server version must match the pinned gremlin-go client minor: the 3.8 driver
# emits the discard() step (renamed from none() in TinkerPop 3.8.0) from
# Iterate(), which 3.7.x servers cannot execute.
FROM tinkerpop/gremlin-server:3.8.1

# Set working directory
WORKDIR /opt/gremlin-server

# Copy custom configuration files
COPY gremlin-config/ /opt/gremlin-server/conf/
# Expose Gremlin server port
EXPOSE 8182
