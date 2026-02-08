/// Configuration for a Vault instance
class Config {
  /// Sync relay server URL (e.g., wss://sync-relay.example.com)
  /// Leave empty to disable sync relay
  final String? syncRelay;

  /// How long data is kept (duration in milliseconds)
  final int? retention;

  /// Maximum allowed store size in bytes
  final int? maxStorage;

  /// Time duration of each batch segment (in milliseconds)
  final int? segmentInterval;

  /// Minimum time between two sync operations in milliseconds (default: 5000)
  final int? syncCooldown;

  /// Maximum time to wait for I/O operations in milliseconds (default: 600000 - 10 minutes)
  final int? waitTimeout;

  /// How often to sync files in milliseconds (default: 600000 - 10 minutes)
  final int? filesSyncPeriod;

  /// How often to run housekeeping in milliseconds (default: 3600000 - 1 hour)
  final int? cleanupPeriod;

  /// How often to sync the blockchain in milliseconds (default: 600000 - 10 minutes)
  final int? blockChainSyncPeriod;

  /// Maximum number of concurrent I/O operations (default: 10)
  final int? ioThrottle;

  const Config({
    this.syncRelay,
    this.retention,
    this.maxStorage,
    this.segmentInterval,
    this.syncCooldown,
    this.waitTimeout,
    this.filesSyncPeriod,
    this.cleanupPeriod,
    this.blockChainSyncPeriod,
    this.ioThrottle,
  });

  /// Convert Config to JSON for C API
  Map<String, dynamic> toJson() {
    return {
      if (syncRelay != null && syncRelay!.isNotEmpty) 'syncRelay': syncRelay,
      if (retention != null && retention! > 0) 'retention': retention,
      if (maxStorage != null && maxStorage! > 0) 'maxStorage': maxStorage,
      if (segmentInterval != null && segmentInterval! > 0)
        'segmentInterval': segmentInterval,
      if (syncCooldown != null && syncCooldown! > 0) 'syncCooldown': syncCooldown,
      if (waitTimeout != null && waitTimeout! > 0) 'waitTimeout': waitTimeout,
      if (filesSyncPeriod != null && filesSyncPeriod! > 0)
        'filesSyncPeriod': filesSyncPeriod,
      if (cleanupPeriod != null && cleanupPeriod! > 0)
        'cleanupPeriod': cleanupPeriod,
      if (blockChainSyncPeriod != null && blockChainSyncPeriod! > 0)
        'blockChainSyncPeriod': blockChainSyncPeriod,
      if (ioThrottle != null && ioThrottle! > 0) 'ioThrottle': ioThrottle,
    };
  }
}
