# Load Balancing

*(Documentation In Progress)*

Currently, TQServer operates with a 1:1 mapping between a Route and a Worker Process (or PHP Pool). True load balancing (multiple instances of the same Go worker) is planned for future releases.

For PHP, load balancing is handled internally by the `php-fpm` process manager, which spawns multiple child processes to handle concurrent requests on the same port.
