use strict;
use warnings;
use DynaLoader ();
use JSON::PP ();

# Both orders use the very same objects and RTLD_GLOBAL (DynaLoader flag 1).
# Retain the handles throughout the process. No unload/reload, fork or exec.
my ($order, $extension, $system_zlib, $module_root, $operation) = @ARGV;
$operation //= 'round-trip';
die "usage: ORDER ABS_EXTENSION ABS_LIBZ ABS_MODULE_ROOT [round-trip|load-only]\n"
    unless @ARGV == 4 || @ARGV == 5;
die "unknown operation\n" unless $operation eq 'round-trip' || $operation eq 'load-only';
die "unknown order\n" unless $order eq 'bundled-first' || $order eq 'system-first';
die "absolute paths required\n" if grep { !m{^/} } ($extension, $system_zlib, $module_root);
unshift @INC, "$module_root/blib/lib", "$module_root/blib/arch";
my @paths = $order eq 'bundled-first'
    ? ($extension, $system_zlib) : ($system_zlib, $extension);
my @handles;
for my $path (@paths) {
    my $handle = DynaLoader::dl_load_file($path, 0x01)
        or die "dl_load_file($path): " . DynaLoader::dl_error();
    push @handles, $handle;
}
# XSLoader reuses the existing mapping of the absolute extension path.
require Compress::Raw::Zlib;
die "wrong Perl module loaded\n"
    unless $INC{'Compress/Raw/Zlib.pm'} eq "$module_root/blib/lib/Compress/Raw/Zlib.pm";
print STDERR JSON::PP->new->canonical->encode({
    module_version => $Compress::Raw::Zlib::VERSION,
    zlib_compile => Compress::Raw::Zlib::ZLIB_VERSION(),
    zlib_runtime => Compress::Raw::Zlib::zlib_version(),
}), "\n";
if ($operation eq 'load-only') {
    print JSON::PP->new->canonical->encode({round_trip => JSON::PP::false}), "\n";
    exit 0;
}

my $payload = ("BindWitness: real Perl zlib round trip\0\1" x 64);
my ($deflater, $status) = Compress::Raw::Zlib::Deflate->new(-AppendOutput => 1);
die "deflate init: $status\n" unless $status == Compress::Raw::Zlib::Z_OK();
my $compressed = '';
$status = $deflater->deflate($payload, $compressed);
die "deflate: $status\n" unless $status == Compress::Raw::Zlib::Z_OK();
$status = $deflater->flush($compressed, Compress::Raw::Zlib::Z_FINISH());
die "flush: $status\n" unless $status == Compress::Raw::Zlib::Z_OK();
my ($inflater, $init) = Compress::Raw::Zlib::Inflate->new(-AppendOutput => 1);
die "inflate init: $init\n" unless $init == Compress::Raw::Zlib::Z_OK();
my $decoded = '';
$status = $inflater->inflate($compressed, $decoded);
die "inflate: $status\n" unless $status == Compress::Raw::Zlib::Z_STREAM_END();
die "round trip mismatch\n" unless $decoded eq $payload;
print JSON::PP->new->canonical->encode({
    round_trip => JSON::PP::true,
    payload_hex => unpack('H*', $payload),
}), "\n";
