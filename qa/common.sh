PC_AWK_PROG=awk
export PC_STDERR=""

_is_mac() {
    if [ "$(uname)" = "Darwin" ]; then
        return 0
    fi
    return 1
}

case `pwd`
in
    */qa)
    	export PATH=.:`cd ..; pwd`:$PATH
	;;
    *)
	echo "Warning: not in qa directory, I'm confused"
	pwd
	status=1
	exit
	;;
esac
