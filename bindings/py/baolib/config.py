"""Configuration for a Vault instance"""


class Config:
    """Configuration for a Bao Vault
    
    All time-based parameters are in milliseconds.
    
    Attributes:
        sync_relay: Sync relay server URL (e.g., wss://sync-relay.example.com).
                   Leave empty to disable sync relay.
        retention: How long data is kept (in milliseconds).
        max_storage: Maximum allowed store size in bytes.
        segment_interval: Time duration of each batch segment (in milliseconds).
        sync_cooldown: Minimum time between two sync operations (default: 5000ms).
        wait_timeout: Maximum time to wait for I/O operations (default: 600000ms - 10 minutes).
        files_sync_period: How often to sync files (default: 600000ms - 10 minutes).
        cleanup_period: How often to run housekeeping (default: 3600000ms - 1 hour).
        block_chain_sync_period: How often to sync the blockchain (default: 600000ms - 10 minutes).
        io_throttle: Maximum number of concurrent I/O operations (default: 10).
    """
    
    def __init__(
        self,
        sync_relay: str = "",
        retention: int = 0,
        max_storage: int = 0,
        segment_interval: int = 0,
        sync_cooldown: int = 0,
        wait_timeout: int = 0,
        files_sync_period: int = 0,
        cleanup_period: int = 0,
        block_chain_sync_period: int = 0,
        io_throttle: int = 0,
    ):
        """Initialize Vault configuration
        
        Args:
            sync_relay: Sync relay server URL (default: empty, disabled)
            retention: Data retention duration in milliseconds (default: 0, no limit)
            max_storage: Maximum store size in bytes (default: 0, no limit)
            segment_interval: Batch segment duration in milliseconds (default: 0)
            sync_cooldown: Minimum time between syncs in milliseconds (default: 0)
            wait_timeout: Max wait time for I/O in milliseconds (default: 0)
            files_sync_period: File sync interval in milliseconds (default: 0)
            cleanup_period: Housekeeping interval in milliseconds (default: 0)
            block_chain_sync_period: Blockchain sync interval in milliseconds (default: 0)
            io_throttle: Concurrent I/O operations limit (default: 0)
        """
        self.sync_relay = sync_relay
        self.retention = retention
        self.max_storage = max_storage
        self.segment_interval = segment_interval
        self.sync_cooldown = sync_cooldown
        self.wait_timeout = wait_timeout
        self.files_sync_period = files_sync_period
        self.cleanup_period = cleanup_period
        self.block_chain_sync_period = block_chain_sync_period
        self.io_throttle = io_throttle
    
    def to_dict(self) -> dict:
        """Convert Config to dictionary for C API"""
        result = {}
        if self.sync_relay:
            result['syncRelay'] = self.sync_relay
        if self.retention > 0:
            result['retention'] = self.retention
        if self.max_storage > 0:
            result['maxStorage'] = self.max_storage
        if self.segment_interval > 0:
            result['segmentInterval'] = self.segment_interval
        if self.sync_cooldown > 0:
            result['syncCooldown'] = self.sync_cooldown
        if self.wait_timeout > 0:
            result['waitTimeout'] = self.wait_timeout
        if self.files_sync_period > 0:
            result['filesSyncPeriod'] = self.files_sync_period
        if self.cleanup_period > 0:
            result['cleanupPeriod'] = self.cleanup_period
        if self.block_chain_sync_period > 0:
            result['blockChainSyncPeriod'] = self.block_chain_sync_period
        if self.io_throttle > 0:
            result['ioThrottle'] = self.io_throttle
        return result
