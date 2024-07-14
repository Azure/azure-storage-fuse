#include "aznfsc.h"
#include "readahead.h"

/*
 * This enables debug logs and also runs the self tests.
 * Must enable once after adding a new self-test.
 */
//#define DEBUG_READAHEAD

#ifndef DEBUG_READAHEAD
#undef AZLogInfo
#undef AZLogDebug
#define AZLogInfo(fmt, ...)     /* nothing */
#define AZLogDebug(fmt, ...)    /* nothing */
#else
/*
 * Debug is not enabled early on when self-tests run, so use Info.
 * Uncomment these if you want to see debug logs from cache self-test.
 */
//#undef AZLogDebug
//#define AZLogDebug AZLogInfo
#endif

#define _MiB (1024 * 1024ULL)
#define _GiB (_MiB * 1024)
#define _TiB (_GiB * 1024)

namespace aznfsc {

/* static */
int ra_state::unit_test()
{
    ra_state ras{0, 128 * 1024};
    uint64_t next_ra;
    uint64_t next_read;
    uint64_t complete_ra;

    AZLogInfo("Unit testing ra_state, start");

    // 1st read.
    next_read = 0*_MiB;
    ras.on_application_read(next_read, 1*_MiB);

    // Only 1 read complete, cannot confirm sequential pattern till 3 reads.
    assert(ras.get_next_ra(4*_MiB) == 0);

    next_read += 1*_MiB;
    ras.on_application_read(next_read, 1*_MiB);

    // Only 2 reads complete, cannot confirm sequential pattern till 3 reads.
    assert(ras.get_next_ra(4*_MiB) == 0);

    next_read += 1*_MiB;
    ras.on_application_read(next_read, 1*_MiB);

    /*
     * Ok 3 reads complete, all were sequential, so now we should get a
     * readahead recommendation.
     */
    next_ra = 3*_MiB;
    assert(ras.get_next_ra(4*_MiB) == next_ra);

    /*
     * Since we have 128MB ra window, next 31 (+1 above) get_next_ra() calls
     * will recommend readahead.
     */
    for (int i = 0; i < 31; i++) {
        next_ra += 4*_MiB;
        assert(ras.get_next_ra(4*_MiB) == next_ra);
    }

    // No more readahead reads after full ra window is issued.
    assert(ras.get_next_ra(4*_MiB) == 0);

    // Complete one readahead.
    complete_ra = 3*_MiB;
    ras.on_readahead_complete(complete_ra, 4*_MiB);

    // One more readahead should be allowed.
    next_ra += 4*_MiB;
    assert(ras.get_next_ra(4*_MiB) == next_ra);

    // Not any more.
    assert(ras.get_next_ra(4*_MiB) == 0);

    // Complete all readahead reads.
    for (int i = 0; i < 32; i++) {
        complete_ra += 4*_MiB;
        ras.on_readahead_complete(complete_ra, 4*_MiB);
    }

    // Now it should recommend next readahead.
    next_ra += 4*_MiB;
    assert(ras.get_next_ra(4*_MiB) == next_ra);

    // Complete that one too.
    complete_ra = next_ra;
    ras.on_readahead_complete(complete_ra, 4*_MiB);

    /*
     * Now issue next read at 100MB offset.
     * This will cause access density to drop since now we have a gap of
     * 97MiB and we have just read 4MiB till now.
     */
    ras.on_application_read(100*_MiB, 1*_MiB);
    assert(ras.get_next_ra(4*_MiB) == 0);

    /*
     * Read the entire gap.
     * This will fill the gap and get the access density back to 100%, so
     * now it should recommend readahead.
     */
    for (int i = 0; i < 97; i++) {
        next_read += 1*_MiB;
        ras.on_application_read(next_read, 1*_MiB);
    }

    /*
     * Readahead recommended should be after the last byte read or the last
     * readahead byte, whichever is larger. In this case next readahead is
     * larger.
     */
    next_ra += 4*_MiB;
    assert(next_ra > 101*_MiB);
    assert(ras.get_next_ra(4*_MiB) == next_ra);

    /*
     * Read from a new section.
     * This should reset the pattern detector and it should not recommend a
     * readahead, till it again confirms a sequential pattern.
     */
    next_read = 2*_GiB;
    ras.on_application_read(next_read, 1*_MiB);
    assert(ras.get_next_ra(4*_MiB) == 0);

    // 2nd read in the new section.
    next_read += 1*_MiB;
    ras.on_application_read(next_read, 1*_MiB);
    assert(ras.get_next_ra(4*_MiB) == 0);

    // 3rd read in the new section.
    next_read += 1*_MiB;
    ras.on_application_read(next_read, 1*_MiB);

    next_ra = next_read + 1*_MiB;
    assert(ras.get_next_ra(4*_MiB) == next_ra);

    /*
     * Read from the next section. We will only do random reads so pattern
     * detector should not see a seq pattern and must not recommend readahead.
     */
    next_read = 4*_GiB;
    ras.on_application_read(next_read, 1*_MiB);
    assert(ras.get_next_ra(4*_MiB) == 0);

    for (int i = 0; i < 1000; i++) {
        next_read = random_number(0, 1*_TiB);
        ras.on_application_read(next_read, 1*_MiB);
        assert(ras.get_next_ra(4*_MiB) == 0);

        next_read = random_number(1*_TiB, 2*_TiB);
        ras.on_application_read(next_read, 1*_MiB);
        assert(ras.get_next_ra(4*_MiB) == 0);
    }

    /*
     * Jump to a new section.
     * Here we will only do sequential reads. After 3 sequential reads, we
     * should detect the pattern and after that we should recommend readahead.
     */
    next_read = 10*_GiB;
    ras.on_application_read(next_read, 1*_MiB);
    assert(ras.get_next_ra(4*_MiB) == 0);

    next_read += 1*_MiB;
    ras.on_application_read(next_read, 1*_MiB);
    assert(ras.get_next_ra(4*_MiB) == 0);

    next_read += 1*_MiB;
    ras.on_application_read(next_read, 1*_MiB);

    next_ra = next_read+1*_MiB;
    assert(ras.get_next_ra(4*_MiB) == next_ra);

    for (int i = 0; i < 2000; i++) {
        next_read += 1*_MiB;
        ras.on_application_read(next_read, 1*_MiB);

        next_ra += 4*_MiB;
        assert(ras.get_next_ra(4*_MiB) == next_ra);

        complete_ra = next_ra;
        ras.on_readahead_complete(complete_ra, 4*_MiB);
    }

    // Stress run.
    for (int i = 0; i < 10'000'000; i++) {
        next_read = random_number(0, 1*_TiB);
        ras.on_application_read(next_read, 1*_MiB);
        assert(!ras.is_sequential());
        assert(ras.get_next_ra(4*_MiB) == 0);

        next_read = random_number(1*_TiB, 2*_TiB);
        ras.on_application_read(next_read, 1*_MiB);
        assert(!ras.is_sequential());
        assert(ras.get_next_ra(4*_MiB) == 0);

        next_read = random_number(2*_TiB, 3*_TiB);
        ras.on_application_read(next_read, 1*_MiB);
        assert(!ras.is_sequential());
        assert(ras.get_next_ra(4*_MiB) == 0);

        next_read = random_number(3*_TiB, 4*_TiB);
        ras.on_application_read(next_read, 1*_MiB);
        assert(!ras.is_sequential());
        assert(ras.get_next_ra(4*_MiB) == 0);
    }

    AZLogInfo("Unit testing ra_state, done!");

    return 0;
}

#ifdef DEBUG_READAHEAD
static int _i = ra_state::unit_test();
#endif

}
