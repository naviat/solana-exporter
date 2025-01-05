package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
    // Node Status Metrics
    NodeHealth     *prometheus.GaugeVec   // Health status (1=healthy, 0=unhealthy)
    NodeVersion    *prometheus.GaugeVec   // Node version info
    
    // Slot/Epoch Metrics
    CurrentSlot    *prometheus.GaugeVec   // Current processed slot
    NetworkSlot    *prometheus.GaugeVec   // Reference node's slot
    SlotBehind     *prometheus.GaugeVec   // How many slots behind reference node
    ConfirmedSlot  *prometheus.GaugeVec   // Confirmed slot height
    SlotHeight     *prometheus.GaugeVec   // Current slot height
    
    // Epoch Details
    EpochInfo      *prometheus.GaugeVec   // Current epoch information
    EpochProgress  *prometheus.GaugeVec   // Epoch progress percentage
    SlotOffset     *prometheus.GaugeVec   // Slot offset within epoch
    SlotsRemaining *prometheus.GaugeVec   // Slots remaining in epoch
    ConfirmedEpochNumber      *prometheus.GaugeVec   // Current epoch number
    ConfirmedEpochFirstSlot   *prometheus.GaugeVec   // First slot of current epoch
    ConfirmedEpochLastSlot    *prometheus.GaugeVec   // Last slot of current epoch
    
    // Block Metrics
    BlockHeight    *prometheus.GaugeVec   // Current block height
    BlockTime      *prometheus.GaugeVec   // Time since last block

    // Process Metrics
    ProcessCPUSeconds        *prometheus.CounterVec  // Total CPU time spent
    ProcessStartTime         *prometheus.GaugeVec    // Process start time
    ProcessVirtualMemory     *prometheus.GaugeVec    // Virtual memory size
    ProcessResidentMemory    *prometheus.GaugeVec    // Resident memory size
    ProcessMaxVirtualMemory  *prometheus.GaugeVec    // Max virtual memory
    ProcessOpenFDs          *prometheus.GaugeVec     // Open file descriptors
    ProcessMaxFDs           *prometheus.GaugeVec     // Max file descriptors

    // Go Runtime Metrics
    GoInfo                  *prometheus.GaugeVec     // Go version information
    GoThreads               *prometheus.GaugeVec     // Number of OS threads
    GoGoroutines           *prometheus.GaugeVec     // Number of goroutines
    GoGCDuration           *prometheus.SummaryVec    // GC duration
    
    // Go Memory Stats
    GoMemStatsAlloc        *prometheus.GaugeVec     // Number of bytes allocated and in use
    GoMemStatsHeapAlloc    *prometheus.GaugeVec     // Heap bytes allocated and in use
    GoMemStatsSys          *prometheus.GaugeVec     // Bytes obtained from system
    GoMemStatsHeapSys      *prometheus.GaugeVec     // Heap bytes obtained from system
    GoMemStatsHeapIdle     *prometheus.GaugeVec     // Heap bytes waiting to be used
    GoMemStatsHeapInuse    *prometheus.GaugeVec     // Heap bytes in use
    GoMemStatsHeapReleased *prometheus.GaugeVec     // Heap bytes released to OS
    GoMemStatsHeapObjects  *prometheus.GaugeVec     // Number of allocated heap objects
    GoMemStatsGCCPUFraction *prometheus.GaugeVec    // Fraction of CPU time used by GC
    GoMemStatsNextGC       *prometheus.GaugeVec     // Heap size at which next GC will trigger
    GoMemStatsLastGC       *prometheus.GaugeVec     // Time of last GC
    GoMemStatsStackInuse   *prometheus.GaugeVec     // Bytes in use by stack allocator
    GoMemStatsMallocs      *prometheus.CounterVec   // Total number of mallocs
    GoMemStatsFrees        *prometheus.CounterVec   // Total number of frees
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
    m := &Metrics{
        // Node Status Metrics
        NodeHealth: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_is_healthy",
                Help: "Is node healthy",
            },
            []string{"endpoint"},
        ),
        NodeVersion: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_node_version",
                Help: "Node version information",
            },
            []string{"endpoint", "version"},
        ),

        // Slot/Epoch Metrics
        CurrentSlot: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_current_slot",
                Help: "Current processed slot number",
            },
            []string{"endpoint", "commitment"},
        ),
        NetworkSlot: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_network_slot",
                Help: "Network's highest known slot",
            },
            []string{"endpoint"},
        ),
        SlotBehind: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_slot_behind",
                Help: "Number of slots behind the reference node",
            },
            []string{"endpoint"},
        ),
        ConfirmedSlot: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_confirmed_slot_height",
                Help: "Last confirmed slot height processed",
            },
            []string{"endpoint"},
        ),
        SlotHeight: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_slot_height",
                Help: "Current slot height",
            },
            []string{"endpoint"},
        ),

        // Epoch Details
        EpochInfo: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_epoch",
                Help: "Current epoch information",
            },
            []string{"endpoint"},
        ),
        EpochProgress: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_epoch_progress",
                Help: "Current epoch progress percentage",
            },
            []string{"endpoint"},
        ),
        SlotOffset: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_slot_offset",
                Help: "Slot offset within current epoch",
            },
            []string{"endpoint"},
        ),
        SlotsRemaining: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_slots_remaining",
                Help: "Slots remaining in current epoch",
            },
            []string{"endpoint"},
        ),
        ConfirmedEpochNumber: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_confirmed_epoch_number",
                Help: "Current epoch",
            },
            []string{"endpoint"},
        ),
        ConfirmedEpochFirstSlot: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_confirmed_epoch_first_slot",
                Help: "Current epoch's first slot",
            },
            []string{"endpoint"},
        ),
        ConfirmedEpochLastSlot: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_confirmed_epoch_last_slot",
                Help: "Current epoch's last slot",
            },
            []string{"endpoint"},
        ),

        // Block Metrics
        BlockHeight: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_block_height",
                Help: "Current block height",
            },
            []string{"endpoint"},
        ),
        BlockTime: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_block_time",
                Help: "Time since last block in seconds",
            },
            []string{"endpoint"},
        ),

        // Process Metrics
        ProcessCPUSeconds: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "process_cpu_seconds_total",
                Help: "Total user and system CPU time spent in seconds",
            },
            []string{"endpoint"},
        ),
        ProcessStartTime: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "process_start_time_seconds",
                Help: "Start time of the process since unix epoch in seconds",
            },
            []string{"endpoint"},
        ),
        ProcessVirtualMemory: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "process_virtual_memory_bytes",
                Help: "Virtual memory size in bytes",
            },
            []string{"endpoint"},
        ),
        ProcessResidentMemory: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "process_resident_memory_bytes",
                Help: "Resident memory size in bytes",
            },
            []string{"endpoint"},
        ),
        ProcessMaxVirtualMemory: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "process_virtual_memory_max_bytes",
                Help: "Maximum amount of virtual memory available in bytes",
            },
            []string{"endpoint"},
        ),
        ProcessOpenFDs: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "process_open_fds",
                Help: "Number of open file descriptors",
            },
            []string{"endpoint"},
        ),
        ProcessMaxFDs: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "process_max_fds",
                Help: "Maximum number of open file descriptors",
            },
            []string{"endpoint"},
        ),

        // Go Runtime Metrics
        GoInfo: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_info",
                Help: "Information about the Go environment",
            },
            []string{"endpoint", "version"},
        ),
        GoThreads: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_threads",
                Help: "Number of OS threads created",
            },
            []string{"endpoint"},
        ),
        GoGoroutines: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_goroutines",
                Help: "Number of goroutines that currently exist",
            },
            []string{"endpoint"},
        ),
        GoGCDuration: prometheus.NewSummaryVec(
            prometheus.SummaryOpts{
                Name: "go_gc_duration_seconds",
                Help: "A summary of the pause duration of garbage collection cycles",
                Objectives: map[float64]float64{
                    0.0:  0.01,
                    0.25: 0.01,
                    0.5:  0.01,
                    0.75: 0.01,
                    1.0:  0.01,
                },
            },
            []string{"endpoint"},
        ),

        // Go Memory Stats
        GoMemStatsAlloc: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_memstats_alloc_bytes",
                Help: "Number of bytes allocated and still in use",
            },
            []string{"endpoint"},
        ),
        GoMemStatsHeapAlloc: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_memstats_heap_alloc_bytes",
                Help: "Number of heap bytes allocated and still in use",
            },
            []string{"endpoint"},
        ),
        GoMemStatsSys: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_memstats_sys_bytes",
                Help: "Number of bytes obtained from system",
            },
            []string{"endpoint"},
        ),
        GoMemStatsHeapSys: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_memstats_heap_sys_bytes",
                Help: "Number of heap bytes obtained from system",
            },
            []string{"endpoint"},
        ),
        GoMemStatsHeapIdle: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_memstats_heap_idle_bytes",
                Help: "Number of heap bytes waiting to be used",
            },
            []string{"endpoint"},
        ),
        GoMemStatsHeapInuse: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_memstats_heap_inuse_bytes",
                Help: "Number of heap bytes that are in use",
            },
            []string{"endpoint"},
        ),
        GoMemStatsHeapReleased: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_memstats_heap_released_bytes",
                Help: "Number of heap bytes released to OS",
            },
            []string{"endpoint"},
        ),
        GoMemStatsHeapObjects: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_memstats_heap_objects",
                Help: "Number of allocated objects",
            },
            []string{"endpoint"},
        ),
        GoMemStatsGCCPUFraction: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_memstats_gc_cpu_fraction",
                Help: "The fraction of this program's available CPU time used by the GC since the program started",
            },
            []string{"endpoint"},
        ),
        GoMemStatsNextGC: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_memstats_next_gc_bytes",
                Help: "Number of heap bytes when next garbage collection will take place",
            },
            []string{"endpoint"},
        ),
        GoMemStatsLastGC: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_memstats_last_gc_time_seconds",
                Help: "Number of seconds since 1970 of last garbage collection",
            },
            []string{"endpoint"},
        ),
        GoMemStatsStackInuse: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "go_memstats_stack_inuse_bytes",
                Help: "Number of bytes in use by the stack allocator",
            },
            []string{"endpoint"},
        ),
        GoMemStatsMallocs: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "go_memstats_mallocs_total",
                Help: "Total number of mallocs",
            },
            []string{"endpoint"},
        ),
        GoMemStatsFrees: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "go_memstats_frees_total",
                Help: "Total number of frees",
            },
            []string{"endpoint"},
        ),
    }

    // Register all metrics
    reg.MustRegister(
        // Node Status Metrics
        m.NodeHealth,
        m.NodeVersion,
        
        // Slot/Epoch Metrics
        m.CurrentSlot,
        m.NetworkSlot,
        m.SlotBehind,
        m.ConfirmedSlot,
        m.SlotHeight,
        m.EpochInfo,
        m.EpochProgress,
        m.SlotOffset,
        m.SlotsRemaining,
        m.ConfirmedEpochNumber,
        m.ConfirmedEpochFirstSlot,
        m.ConfirmedEpochLastSlot,
        
        // Block Metrics
        m.BlockHeight,
        m.BlockTime,

        // Process Metrics
        m.ProcessCPUSeconds,
        m.ProcessStartTime,
        m.ProcessVirtualMemory,
        m.ProcessResidentMemory,
        m.ProcessMaxVirtualMemory,
        m.ProcessOpenFDs,
        m.ProcessMaxFDs,

        // Go Runtime Metrics
        m.GoInfo,
        m.GoThreads,
        m.GoGoroutines,
        m.GoGCDuration,

        // Go Memory Stats
        m.GoMemStatsAlloc,
        m.GoMemStatsHeapAlloc,
        m.GoMemStatsSys,
        m.GoMemStatsHeapSys,
        m.GoMemStatsHeapIdle,
        m.GoMemStatsHeapInuse,
        m.GoMemStatsHeapReleased,
        m.GoMemStatsHeapObjects,
        m.GoMemStatsGCCPUFraction,
        m.GoMemStatsNextGC,
        m.GoMemStatsLastGC,
        m.GoMemStatsStackInuse,
        m.GoMemStatsMallocs,
        m.GoMemStatsFrees,
    )

    return m
}
